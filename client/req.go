package client

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	token "github.com/bmeg/grip-graphql/middleware"
	"github.com/calypr/data-client/data-client/commonUtils"
	"github.com/calypr/data-client/data-client/jwt"
)

type Resp struct {
	Body []byte
	Err  error
}

func (cl *FenceOBJ) makeGen3Req(method string, path string) *Resp {
	a := *cl.Base
	a.Path = filepath.Join(a.Path, path)

	req, err := http.NewRequest(method, a.String(), nil)
	if err != nil {
		return &Resp{nil, err}
	}
	expiration, err := token.GetExpiration(cl.Cred.AccessToken)
	if err != nil {
		return &Resp{nil, err}
	}
	// Update AccessToken if token is old
	if expiration.Before(time.Now()) {
		r := jwt.Request{}
		r.RequestNewAccessToken(cl.Base.String()+commonUtils.FenceAccessTokenEndpoint, &cl.Cred)
	}
	if cl.Cred.AccessToken == "" {
		return &Resp{nil, fmt.Errorf("access token not found in profile config")}
	}
	req.Header.Set("Authorization", "Bearer "+cl.Cred.AccessToken)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return &Resp{nil, err}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return &Resp{nil, fmt.Errorf("failed to check authz, response body: %v", response)}
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return &Resp{nil, fmt.Errorf("failed to read response Body")}
	}

	return &Resp{body, nil}
}
