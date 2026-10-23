package selfupdate

type Source interface {
	LatestRelease() (Release, error)
	Download(url string) ([]byte, error)
}

type Installer interface {
	Install(data []byte) error
}
