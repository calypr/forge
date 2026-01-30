package client

import (
	"github.com/calypr/git-drs/client"
	drsConfig "github.com/calypr/git-drs/config"

	"github.com/calypr/data-client/g3client"
	"github.com/calypr/data-client/logs"
)

// NewGen3Client loads repo-level config and return a new DRSClient
func NewGen3Client(remote drsConfig.Remote, opts ...g3client.Option) (client.DRSClient, func(), error) {
	cfg, err := drsConfig.LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	logger, closer := logs.New(string(remote))

	dClient, err := cfg.GetRemoteClient(remote, logger.Logger, opts...)
	if err != nil {
		return nil, closer, err
	}

	return dClient, closer, nil
}
