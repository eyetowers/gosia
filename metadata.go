package sia

// Metadata is a SIA-DCS metadata field code.
type Metadata string

const (
	verification Metadata = "V"
	longitude    Metadata = "X"
	latitude     Metadata = "Y"
	altitude     Metadata = "Z"
)
