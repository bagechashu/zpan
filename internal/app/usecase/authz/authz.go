package authz

import (
	"context"
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/open-policy-agent/opa/rego"
	"github.com/saltbo/zpan/internal/pkg/authed"
	"github.com/spf13/viper"
)

type Config struct {
	ShareAllFiles bool `json:"share_all_files"`
}

type Input struct {
	Uid        int64       `json:"uid"`
	Path       string      `json:"path"`
	Method     string      `json:"method"`
	PathParams []gin.Param `json:"path_params"`
	Resource   any         `json:"resource"`
	Config     Config      `json:"config"`
}

//go:embed authz.rego
var module string

func NewMiddleware(c *gin.Context) {
	bw := NewWriter(c.Writer)
	c.Writer = bw
	c.Next()

	input := &Input{
		Uid:        authed.UidGet(c),
		Path:       c.FullPath(),
		Method:     c.Request.Method,
		PathParams: c.Params,
		Resource:   bw.extractResource(),
		Config: Config{
			ShareAllFiles: viper.GetBool("share.all_files"),
		},
	}

	if rs, err := Decision(c, input); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	} else if !rs.Allowed() {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	bw.WriteNow()
}

func Decision(ctx context.Context, input *Input) (rego.ResultSet, error) {
	r := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("./authz.rego", module),
	)

	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return query.Eval(ctx, rego.EvalInput(input))
}
