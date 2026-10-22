package agent

type Discoverer interface {
	Discover(projectDir string) ([]Agent, error)
}

type Catalog struct {
	discoverer Discoverer
}

func NewCatalog(d Discoverer) *Catalog {
	return &Catalog{discoverer: d}
}

func (c *Catalog) Discover(projectDir string) ([]Agent, []error) {
	agents, err := c.discoverer.Discover(projectDir)
	return agents, splitJoined(err)
}

func splitJoined(err error) []error {
	if err == nil {
		return nil
	}
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		return u.Unwrap()
	}
	return []error{err}
}
