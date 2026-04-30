package payment

import "encoding/json"

// unmarshalJSON is a thin wrapper around encoding/json.Unmarshal used internally
// by adapters so they do not need to redeclare the import in multiple files.
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
