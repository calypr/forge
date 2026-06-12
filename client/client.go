package client

import (
	"fmt"

	"github.com/calypr/calypr-cli/conf"
	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/calypr-cli/logs"
)

type ProfileClient struct {
	ProfileName string
	Gen3        g3client.Gen3Interface
	Logger      *logs.Gen3Logger
}

func (c *ProfileClient) Profile() string {
	return c.ProfileName
}

func (c *ProfileClient) Credential() *conf.Credential {
	cred := c.Gen3.Credentials().Current()
	if cred == nil {
		return &conf.Credential{}
	}
	return cred
}

func NewGen3Client(profileName string, opts ...g3client.Option) (*ProfileClient, func(), error) {
	logger, closer := logs.New(profileName)
	gen3, err := g3client.NewGen3Interface(profileName, logger, opts...)
	if err != nil {
		closer()
		return nil, nil, fmt.Errorf("failed to initialize Gen3 client for profile %q: %w", profileName, err)
	}

	return &ProfileClient{
		ProfileName: profileName,
		Gen3:        gen3,
		Logger:      logger,
	}, closer, nil
}
