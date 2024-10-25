package t

const PerceptrailPathFieldName string = "__perceptrail_file_path"

type ItemPath string

type RawExif map[string][]byte

type Size struct {
	W int
	H int
}
