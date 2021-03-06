package ocm

import (
	"fmt"
	"time"

	sdkClient "github.com/openshift-online/ocm-sdk-go"
	"github.com/patrickmn/go-cache"
)

type Client struct {
	config *Config
	logger sdkClient.Logger
	Cache  *cache.Cache

	Authentication OCMAuthentication
	Authorization  OCMAuthorization
}

type Config struct {
	BaseURL      string `envconfig:"OCM_BASE_URL" default:""`
	ClientID     string `envconfig:"OCM_SERVICE_CLIENT_ID" default:""`
	ClientSecret string `envconfig:"OCM_SERVICE_CLIENT_SECRET" default:""`
	SelfToken    string `envconfig:"OCM_SELF_TOKEN" default:""`
	Debug        bool   `envconfig:"OCM_DEBUG" default:"false"`
}

func NewClient(config Config) (*Client, error) {
	// Create a logger that has the debug level enabled:
	logger, err := sdkClient.NewGoLoggerBuilder().
		Debug(config.Debug).
		Build()
	if err != nil {
		return nil, fmt.Errorf("Unable to build OCM logger: %s", err.Error())
	}
	client := &Client{
		config: &config,
		logger: logger,
		Cache:  cache.New(1*time.Minute, 30*time.Minute),
	}
	client.Authentication = &authentication{
		client: client,
	}
	client.Authorization = &authorization{
		client: client,
	}
	return client, nil
}

// NewConnection creates a new connection
func (c *Client) NewConnection() (*sdkClient.Connection, error) {
	builder := sdkClient.NewConnectionBuilder().
		Logger(c.logger).
		URL(c.config.BaseURL).
		Metrics("api_outbound")

	if c.config.ClientID != "" && c.config.ClientSecret != "" {
		builder = builder.Client(c.config.ClientID, c.config.ClientSecret)
	} else if c.config.SelfToken != "" {
		builder = builder.Tokens(c.config.SelfToken)
	} else {
		return nil, fmt.Errorf("Can't build OCM client connection. No Client/Secret or Token has been provided")
	}

	connection, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("Can't build OCM client connection: %s", err.Error())
	}
	return connection, nil
}

// AuthPayload defines the structure of the User
type AuthPayload struct {
	Username     string `json:"username"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Organization string `json:"org_id"`
	Email        string `json:"email"`
	Issuer       string `json:"iss"`
	ClientID     string `json:"clientId"`
	IsAdmin      bool   `json:"is_admin"`
	IsUser       bool   `json:"is_user"`
}
