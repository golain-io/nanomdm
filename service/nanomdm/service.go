// Pacakge nanomdm is an MDM service.
package nanomdm

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/micromdm/nanomdm/mdm"
	"github.com/micromdm/nanomdm/push"
	"github.com/micromdm/nanomdm/service"
	"github.com/micromdm/nanomdm/storage"

	"github.com/micromdm/plist"
	"github.com/micromdm/nanolib/log"
	"github.com/micromdm/nanolib/log/ctxlog"
)

type autoDeviceInformation struct {
	enqueuer  storage.CommandEnqueuer
	pusher    push.Pusher
	infoStore storage.DeviceInfoStore
}

// Service is the main NanoMDM service which dispatches to storage.
type Service struct {
	logger     log.Logger
	normalizer func(e *mdm.Enrollment) *mdm.EnrollID
	store      storage.ServiceStore

	// Declarative Management
	dm service.DeclarativeManagement

	// UserAuthenticate processor
	ua service.UserAuthenticate

	// GetToken handler
	gt service.GetToken

	// Optional: auto-enqueue DeviceInformation after TokenUpdate
	autoDevInfo *autoDeviceInformation
}

func normalize(e *mdm.Enrollment) *mdm.EnrollID {
	r := e.Resolved()
	if r == nil {
		return nil
	}
	eid := &mdm.EnrollID{
		Type: r.Type,
		ID:   r.DeviceChannelID,
	}
	if r.IsUserChannel {
		eid.ID += ":" + r.UserChannelID
		eid.ParentID = r.DeviceChannelID
	}
	return eid
}

type Option func(*Service)

func WithLogger(logger log.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithDeclarativeManagement(dm service.DeclarativeManagement) Option {
	return func(s *Service) {
		s.dm = dm
	}
}

// WithUserAuthenticate configures a UserAuthenticate check-in message handler.
func WithUserAuthenticate(ua service.UserAuthenticate) Option {
	return func(s *Service) {
		s.ua = ua
	}
}

// WithGetToken configures a GetToken check-in message handler.
func WithGetToken(gt service.GetToken) Option {
	return func(s *Service) {
		s.gt = gt
	}
}

func WithAutoDeviceInformation(enqueuer storage.CommandEnqueuer, pusher push.Pusher, infoStore storage.DeviceInfoStore) Option {
	return func(s *Service) {
		s.autoDevInfo = &autoDeviceInformation{
			enqueuer:  enqueuer,
			pusher:    pusher,
			infoStore: infoStore,
		}
	}
}

// New returns a new NanoMDM main service.
func New(store storage.ServiceStore, opts ...Option) *Service {
	nanomdm := &Service{
		store:      store,
		logger:     log.NopLogger,
		normalizer: normalize,
	}
	for _, opt := range opts {
		opt(nanomdm)
	}
	return nanomdm
}

func encodePlist(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := plist.NewEncoder(buf)
	enc.Indent("\t")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) maybeEnqueueAutoDeviceInformation(r *mdm.Request) {
	if s.autoDevInfo == nil || s.autoDevInfo.enqueuer == nil {
		return
	}
	if r.ParentID != "" {
		return
	}

	logger := ctxlog.Logger(r.Context(), s.logger).With("component", "auto-device-info")

	if s.autoDevInfo.infoStore != nil {
		if info, err := s.autoDevInfo.infoStore.RetrieveDeviceInfo(r.Context(), r.ID); err == nil && info != nil && info.LastDeviceQuery != "" {
			logger.Debug("msg", "skipping auto deviceinformation; already have metadata", "id", r.ID, "last_device_query", info.LastDeviceQuery)
			return
		}
	}

	suffix, err := randomHex(8)
	if err != nil {
		logger.Debug("msg", "generating command uuid", "err", err)
		return
	}
	cmdUUID := "auto-device-info-" + suffix

	cmd := &mdm.Command{
		CommandUUID: cmdUUID,
		Command: struct{ RequestType string }{
			RequestType: "DeviceInformation",
		},
	}
	raw, err := encodePlist(cmd)
	if err != nil {
		logger.Debug("msg", "encoding deviceinformation plist", "err", err)
		return
	}
	cmd.Raw = raw

	perIDErrs, err := s.autoDevInfo.enqueuer.EnqueueCommand(r.Context(), []string{r.ID}, cmd)
	if err != nil {
		logger.Info("msg", "enqueue deviceinformation", "id", r.ID, "command_uuid", cmdUUID, "err", err)
		return
	}
	if perIDErrs != nil {
		if idErr, ok := perIDErrs[r.ID]; ok && idErr != nil {
			logger.Info("msg", "enqueue deviceinformation", "id", r.ID, "command_uuid", cmdUUID, "err", idErr)
			return
		}
	}

	logger.Info("msg", "auto-enqueued deviceinformation", "id", r.ID, "command_uuid", cmdUUID)

	// Best-effort push so the device checks in quickly.
	if s.autoDevInfo.pusher != nil {
		if res, err := s.autoDevInfo.pusher.Push(r.Context(), []string{r.ID}); err != nil {
			logger.Info("msg", "auto-push after deviceinformation enqueue", "id", r.ID, "command_uuid", cmdUUID, "err", err)
		} else if res != nil {
			if r0, ok := res[r.ID]; ok && r0 != nil && r0.Err != nil {
				logger.Info("msg", "auto-push after deviceinformation enqueue", "id", r.ID, "command_uuid", cmdUUID, "err", r0.Err)
			} else {
				logger.Debug("msg", "auto-push after deviceinformation enqueue", "id", r.ID, "command_uuid", cmdUUID)
			}
		}
	}
}

func (s *Service) setupRequest(r *mdm.Request, e *mdm.Enrollment) (*mdm.Request, error) {
	if r.EnrollID != nil && r.ID != "" {
		ctxlog.Logger(r.Context(), s.logger).Debug(
			"msg", "overwriting enrollment id",
		)
	}
	r.EnrollID = s.normalizer(e)
	if err := r.EnrollID.Validate(); err != nil {
		return r, err
	}
	ctx := newContextWithValues(r.Context(), r)
	ctx = ctxlog.AddFunc(ctx, ctxKVs)
	return r.WithContext(ctx), nil
}

// Authenticate Check-in message implementation.
func (s *Service) Authenticate(r *mdm.Request, message *mdm.Authenticate) error {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return err
	}
	logs := []interface{}{
		"msg", "Authenticate",
	}
	if message.SerialNumber != "" {
		logs = append(logs, "serial_number", message.SerialNumber)
	}
	ctxlog.Logger(r.Context(), s.logger).Info(logs...)
	if err := s.store.StoreAuthenticate(r, message); err != nil {
		return err
	}
	// clear the command queue for any enrollment or sub-enrollment.
	// this prevents queued commands still being queued after device
	// unenrollment.
	if err := s.store.ClearQueue(r); err != nil {
		return err
	}
	// then, disable the enrollment or any sub-enrollment (because an
	// enrollment is only valid after a tokenupdate)
	return s.store.Disable(r)
}

// TokenUpdate Check-in message implementation.
func (s *Service) TokenUpdate(r *mdm.Request, message *mdm.TokenUpdate) error {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return err
	}
	ctxlog.Logger(r.Context(), s.logger).Info("msg", "TokenUpdate")
	if err := s.store.StoreTokenUpdate(r, message); err != nil {
		return err
	}
	s.maybeEnqueueAutoDeviceInformation(r)
	return nil
}

// CheckOut Check-in message implementation.
func (s *Service) CheckOut(r *mdm.Request, message *mdm.CheckOut) error {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return err
	}
	ctxlog.Logger(r.Context(), s.logger).Info("msg", "CheckOut")
	return s.store.Disable(r)
}

// UserAuthenticate Check-in message implementation
func (s *Service) UserAuthenticate(r *mdm.Request, message *mdm.UserAuthenticate) ([]byte, error) {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return nil, err
	}
	ctxlog.Logger(r.Context(), s.logger).Info(
		"msg", "UserAuthenticate",
		"digest_response", message.DigestResponse != "",
	)
	if s.ua == nil {
		return nil, errors.New("no UserAuthenticate handler")
	}
	return s.ua.UserAuthenticate(r, message)
}

func (s *Service) SetBootstrapToken(r *mdm.Request, message *mdm.SetBootstrapToken) error {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return err
	}
	ctxlog.Logger(r.Context(), s.logger).Info("msg", "SetBootstrapToken")
	return s.store.StoreBootstrapToken(r, message)
}

func (s *Service) GetBootstrapToken(r *mdm.Request, message *mdm.GetBootstrapToken) (*mdm.BootstrapToken, error) {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return nil, err
	}
	ctxlog.Logger(r.Context(), s.logger).Info("msg", "GetBootstrapToken")
	return s.store.RetrieveBootstrapToken(r, message)
}

// DeclarativeManagement Check-in message implementation. Calls out to
// the service's DM handler (if configured).
func (s *Service) DeclarativeManagement(r *mdm.Request, message *mdm.DeclarativeManagement) ([]byte, error) {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return nil, err
	}
	ctxlog.Logger(r.Context(), s.logger).Info(
		"msg", "DeclarativeManagement",
		"endpoint", message.Endpoint,
	)
	if s.dm == nil {
		return nil, errors.New("no Declarative Management handler")
	}
	return s.dm.DeclarativeManagement(r, message)
}

// GetToken implements the GetToken Check-in message interface.
func (s *Service) GetToken(r *mdm.Request, message *mdm.GetToken) (*mdm.GetTokenResponse, error) {
	r, err := s.setupRequest(r, &message.Enrollment)
	if err != nil {
		return nil, err
	}
	ctxlog.Logger(r.Context(), s.logger).Info(
		"msg", "GetToken",
		"token_service_type", message.TokenServiceType,
	)
	if s.gt == nil {
		return nil, errors.New("no GetToken handler")
	}
	return s.gt.GetToken(r, message)
}

// CommandAndReportResults command report and next-command request implementation.
func (s *Service) CommandAndReportResults(r *mdm.Request, results *mdm.CommandResults) (*mdm.Command, error) {
	r, err := s.setupRequest(r, &results.Enrollment)
	if err != nil {
		return nil, err
	}
	logger := ctxlog.Logger(r.Context(), s.logger)
	logs := []interface{}{
		"status", results.Status,
	}
	if results.CommandUUID != "" {
		logs = append(logs, "command_uuid", results.CommandUUID)
	}
	logger.Info(logs...)
	err = s.store.StoreCommandReport(r, results)
	if err != nil {
		return nil, fmt.Errorf("storing command report: %w", err)
	}
	cmd, err := s.store.RetrieveNextCommand(r, results.Status == "NotNow")
	if err != nil {
		return nil, fmt.Errorf("retrieving next command: %w", err)
	}
	if cmd != nil {
		logger.Debug(
			"msg", "command retrieved",
			"command_uuid", cmd.CommandUUID,
			"request_type", cmd.Command.RequestType,
		)
		return cmd, nil
	}
	logger.Debug(
		"msg", "no command retrieved",
	)
	return nil, nil
}
