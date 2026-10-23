package selfupdate

type Asset struct {
	Name string
	URL  string
}

type Release struct {
	Version string
	Assets  []Asset
}

func (r Release) FindAsset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

func AssetName(goos, goarch string) string {
	return "monday-" + goos + "-" + goarch
}

const ChecksumsAssetName = "checksums.txt"
