package installer

import (
	"encoding/json"
	"os"
)

// WriteTempJSON serializes v as JSON to a temporary file and returns its path.
// The caller is responsible for removing the file when done.
func WriteTempJSON(prefix string, v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp("", prefix+"-*.json")
	if err != nil {
		return "", err
	}

	path := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}

	return path, nil
}

// ReadTempJSON reads a JSON file and unmarshals it into v.
func ReadTempJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
