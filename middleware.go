package tracking

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func GINInsightsTracker(i Insights, IKEY string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := i.StartGinTrackingRequestRequest(c, IKEY)
		c.Set("requestCtx", req)
		c.Next()
		code := strconv.Itoa(c.Writer.Status())
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			i.CompleteSuccesfulRequest(req, code)
		} else {
			i.CompleteFailedRequest(req, code, c.Errors.Last())
		}
	}
}
