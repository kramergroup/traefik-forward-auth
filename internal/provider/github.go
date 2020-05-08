package provider

import (
	"context"
	"encoding/json"
	"errors"

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

	// Return a serialised token because the full token is needed
	// for resolving user information, not just the bearer token
	return serialiseOauthToken(token)

}

// GetLoginURL provides the login url for the given redirect uri and state
func (o *Github) GetLoginURL(redirectURI, state string) string {

	c := o.ConfigCopy(redirectURI)
	return c.AuthCodeURL(state)

}

// GetUser uses the given token and returns a complete provider.User object
func (o *Github) GetUser(token string) (User, error) {

	ctx := context.Background()

	oauthToken, err := deserialiseOauthToken(token)
	if err != nil {
		return User{}, err
	}

	s := CreateGithubAPISession(ctx, oauthToken)
	user, err := s.userDetails()

	// Check organisation
	// TODO This should be part of a separate validation step, but this
	//      would need an API change, e.g., by adding a ValidateToken(token string)
	//      method to the Provider interface, which is then called at the appropriate
	//      place in the request handlers. Alternatively, a separate Validator could be
	//      used to aggregate all validation logic in one place.

	if o.Organization != "" {
		ok, err := s.isMember(user.ID, o.Organization)
		if err != nil || !ok {
			return user, errors.New("User failed organisation membership verification")
		}
	}

	return user, err

}

func (o *Github) validateOrganisation() {}

// Support functions -------------------------------------------------------

// serialiseOauthtoken serialises a token
func serialiseOauthToken(token *oauth2.Token) (string, error) {

	bytes, err := json.Marshal(token)
	return string(bytes), err

}

func deserialiseOauthToken(token string) (*oauth2.Token, error) {

	oauthToken := &oauth2.Token{}
	err := json.Unmarshal([]byte(token), &oauthToken)
	return oauthToken, err

}

// Github API calls -------------------------------------------------------

// GithubAPISession reflects a "short-lived" communication session
// with the Github API using authenticated transport
type GithubAPISession struct {
	ctx    context.Context
	client *github.Client
}

// CreateGithubAPISession instantiates a new GithubAPISession
func CreateGithubAPISession(context context.Context, token *oauth2.Token) *GithubAPISession {

	tp := oauth2.NewClient(context, oauth2.StaticTokenSource(token))
	client := github.NewClient(tp)

	return &GithubAPISession{
		ctx:    context,
		client: client,
	}

}

// getUserDetailsFromGithub queries the Github API for user data
func (s *GithubAPISession) userDetails() (User, error) {

	// Get user object
	githubUser, _, err := s.client.Users.Get(s.ctx, "")
	if err != nil {
		return User{}, err
	}

	// Get all eMails registered for the user
	emails, _, err := s.client.Users.ListEmails(s.ctx, nil)
	if err != nil {
		return User{}, err
	}

	// Use primary email for details
	email := &github.UserEmail{}
	for _, e := range emails {
		if e.GetPrimary() {
			email = e
		}
	}

	return User{
		Email:    email.GetEmail(),
		Verified: email.GetVerified(),
		ID:       githubUser.GetLogin(),
	}, err

}

func (s *GithubAPISession) isMember(user string, organisation string) (bool, error) {
	member, _, err := s.client.Organizations.IsMember(s.ctx, organisation, user)
	return member, err
}
