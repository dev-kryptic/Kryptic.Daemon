package api

import "net/url"

type BundleEntry struct {
	Key           string `json:"key"`
	Envelope      string `json:"envelope"`
	DefinitionId  string `json:"definitionId"`
	EnvironmentId string `json:"environmentId"`
}

type Bundle struct {
	ProjectPublicId string        `json:"projectPublicId"`
	Environment     string        `json:"environment"`
	OrgKeyId        string        `json:"orgKeyId"`
	WrappedOrgKey   string        `json:"wrappedOrgKey"`
	Secrets         []BundleEntry `json:"secrets"`
	DynamicSecrets  []DynamicHint `json:"dynamicSecrets"`
}

type Project struct {
	PublicId     string   `json:"publicId"`
	Name         string   `json:"name"`
	Environments []string `json:"environments"`
}

func (c *Client) Bundle(accessToken, projectPublicId, environment string) (*Bundle, error) {
	var out Bundle
	query := url.Values{"projectPublicId": {projectPublicId}, "environment": {environment}}
	err := c.get("/api/secrets/bundle?"+query.Encode(), accessToken, &out)
	return &out, err
}

func (c *Client) Projects(accessToken string) ([]Project, error) {
	var out []Project
	err := c.get("/api/secrets/projects", accessToken, &out)
	return out, err
}
