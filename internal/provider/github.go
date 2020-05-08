package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	github "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

// Github provider
type Github struct {
	OAuthProvider

	ClientID     string `long:"client-id" env:"CLIENT_ID" description:"Client ID"`
	ClientSecret string `long:"client-secret" env:"CLIENT_SECRET" description:"Client Secret" json:"-"`
	Scope        string
	Organization string `long:"organisation" env:"ORGANISATION" description:"Github Organisation"`

	LoginURL *url.URL
	TokenURL *url.URL
	UserURL  *url.URL
}

// Name returns the name of the provider
func (o *Github) Name() string {
	return "github"
}

// Setup performs validation and setup
func (o *Github) Setup() error {
	// Check parms
	if o.ClientID == "" || o.ClientSecret == "" {
		return errors.New("providers.github.client-id, providers.github.client-secret must be set")
	}

	if o.Organization == "" {
		// TODO: Log a warning that organisation is not set - all github accounts are valid!
	}

	// Create oauth2 config
	o.Config = &oauth2.Config{
		ClientID:     o.ClientID,
		ClientSecret: o.ClientSecret,
		Endpoint:     githuboauth.Endpoint,

		// "read:org" and "user:email" is required
		Scopes: []string{"read:org user:email"},
	}

	o.ctx = context.Background()

	return nil
}

// ExchangeCode exchanges the given redirect uri and code for a tokens
func (o *Github) ExchangeCode(redirectURI, code string) (string, error) {
	token, err := o.OAuthExchangeCode(redirectURI, code)
	if err != nil {
		return "", err
	}
	bytes, err := json.Marshal(token)
	return string(bytes), err
}

// GetLoginURL provides the login url for the given redirect uri and state
func (o *Github) GetLoginURL(redirectURI, state string) string {
	c := o.ConfigCopy(redirectURI)
	return c.AuthCodeURL(state)
}

// GetUser uses the given token and returns a complete provider.User object
func (o *Github) GetUser(token string) (User, error) {

	oauthToken := &oauth2.Token{}

	err := json.Unmarshal([]byte(token), &oauthToken)
	if err != nil {
		return User{}, err
	}

	ctx := context.Background()
	tp := oauth2.NewClient(ctx, oauth2.StaticTokenSource(oauthToken))

	client := github.NewClient(tp)
	githubUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return User{}, err
	}
	emails, _, err := client.Users.ListEmails(ctx, nil)
	if err != nil {
		return User{}, err
	}
	email := &github.UserEmail{}
	for _, e := range emails {
		if e.GetPrimary() {
			email = e
		}
	}

	user := User{Email: email.GetEmail(), Verified: email.GetVerified(), ID: githubUser.GetLogin()}
	return user, nil
}
