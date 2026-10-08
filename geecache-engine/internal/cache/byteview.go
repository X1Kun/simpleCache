package cache

// ByteView wraps an immutable cached byte slice.
type ByteView struct {
	b []byte
}

// Len implements the LRU Value interface.
func (bv ByteView) Len() int {
	return len(bv.b)
}

// ByteSlice returns a copy.
func (bv ByteView) ByteSlice() []byte {
	return cloneBytes(bv.b)
}

// String returns the cached bytes as a string.
func (bv ByteView) String() string {
	return string(bv.b)
}

// cloneBytes copies the input slice.
func cloneBytes(input []byte) []byte {
	output := make([]byte, len(input))
	copy(output, input)
	return output
}
