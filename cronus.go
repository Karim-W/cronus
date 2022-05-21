package tracking

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/karim-w/sonic"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type IncomingRequest struct {
	Logger           *zap.SugaredLogger
	TransactionId    string
	Path             string
	Method           string
	RequestTrace     *appinsights.RequestTelemetry
	RequestStartTime time.Time
	Ikey             string
	Dependencies     []*appinsights.RemoteDependencyTelemetry
	Exceptions       []*appinsights.ExceptionTelemetry
}

func GenerateGinTrackingRequestCtx(ctx *gin.Context, IKEY string) *IncomingRequest {
	tid := ctx.Request.Header.Get("transactionId")
	return &IncomingRequest{
		TransactionId:    tid,
		Path:             ctx.Request.URL.Path,
		Method:           ctx.Request.Method,
		RequestTrace:     appinsights.NewRequestTelemetry(ctx.Request.URL.Path, ctx.Request.Method, time.Nanosecond, ""),
		RequestStartTime: time.Now(),
		Logger:           sonic.TrackingLogger(tid),
		Ikey:             IKEY,
	}
}

func (i *IncomingRequest) Info(args ...interface{}) {
	i.Logger.Info("[", i.TransactionId, "]	", args)
}
func (i *IncomingRequest) Debug(args ...interface{}) {
	i.Logger.Debug("[", i.TransactionId, "]	", args)
}
func (i *IncomingRequest) AddDependency(Name string, Type string, target string) *appinsights.RemoteDependencyTelemetry {
	k := appinsights.NewRemoteDependencyTelemetry(Name, Type, target, false)
	k.Timestamp = time.Now()
	k.Id = uuid.NewString()
	i.Dependencies = append(i.Dependencies, k)
	return k
}
func (i *IncomingRequest) CompleteDependency(dep *appinsights.RemoteDependencyTelemetry, succes bool) {
	dep.Duration = time.Since(dep.Timestamp)
	dep.Success = succes
}

func (i *IncomingRequest) LogFaliure(err error) {
	f := appinsights.NewExceptionTelemetry(err)
	f.Timestamp = time.Now()
	f.Error = err
	f.Properties["errorMessage"] = err.Error()
	f.SeverityLevel = appinsights.Critical
	i.Exceptions = append(i.Exceptions, f)
}

func (i *IncomingRequest) LogWarning(err error) {
	f := appinsights.NewExceptionTelemetry(err)
	f.Timestamp = time.Now()
	f.Error = err
	f.Properties["errorMessage"] = err.Error()
	f.SeverityLevel = appinsights.Warning
	i.Exceptions = append(i.Exceptions, f)
}

type Insights interface {
	StartGinTrackingRequestRequest(ctx *gin.Context, IKEY string) *IncomingRequest
	CompleteSuccesfulRequest(r *IncomingRequest, code string)
	CompleteFailedRequest(r *IncomingRequest, code string, err error)
	handleRemoteDependencies(tid string, deps []*appinsights.RemoteDependencyTelemetry)
	handleExceptions(tid string, deps []*appinsights.ExceptionTelemetry)
}

type insightsImpl struct {
	logger *zap.SugaredLogger
	client appinsights.TelemetryClient
}

var _ Insights = (*insightsImpl)(nil)

func NewInsights(logger *zap.SugaredLogger, Ikey string) Insights {
	client := appinsights.NewTelemetryClient(Ikey)
	return &insightsImpl{
		logger: logger,
		client: client,
	}
}

var InsightFXModule = fx.Option(fx.Provide(NewInsights))

func (i *insightsImpl) StartGinTrackingRequestRequest(ctx *gin.Context, IKEY string) *IncomingRequest {
	r := GenerateGinTrackingRequestCtx(ctx, IKEY)
	r.RequestTrace.Properties["Query"] = ctx.Request.URL.RawQuery
	r.RequestTrace.Timestamp = time.Now()
	r.RequestTrace.Id = r.TransactionId
	return r
}

func (i *insightsImpl) CompleteSuccesfulRequest(r *IncomingRequest, code string) {
	r.RequestTrace.Duration = time.Since(r.RequestStartTime)
	r.RequestTrace.Success = true
	r.RequestTrace.ResponseCode = code
	r.RequestTrace.Tags.Operation().SetId(r.TransactionId)
	i.client.Track(r.RequestTrace)
	i.handleRemoteDependencies(r.RequestTrace.Id, r.Dependencies)
	i.handleExceptions(r.RequestTrace.Id, r.Exceptions)
}

func (i *insightsImpl) CompleteFailedRequest(r *IncomingRequest, code string, err error) {
	r.RequestTrace.Duration = time.Since(r.RequestStartTime)
	r.RequestTrace.Success = false
	r.RequestTrace.ResponseCode = code
	r.RequestTrace.Properties["error"] = err.Error()
	i.client.Track(r.RequestTrace)
	i.handleRemoteDependencies(r.RequestTrace.Id, r.Dependencies)
	i.handleExceptions(r.RequestTrace.Id, r.Exceptions)
}
func (i *insightsImpl) handleRemoteDependencies(tid string, deps []*appinsights.RemoteDependencyTelemetry) {
	for _, dep := range deps {
		dep.Tags.Operation().SetParentId(tid)
		i.client.Track(dep)
	}
}
func (i *insightsImpl) handleExceptions(tid string, deps []*appinsights.ExceptionTelemetry) {
	for _, dep := range deps {
		dep.Tags.Operation().SetParentId(tid)
		i.client.Track(dep)
	}
}
