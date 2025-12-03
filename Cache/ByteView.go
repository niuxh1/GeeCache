package cache

type ByteView struct{
	bt []byte
}

func NewByteView(b []byte) ByteView {
	return ByteView{bt: cloneBytes(b)}
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func (b ByteView) Len() int {
	return len(b.bt)
}

func (b ByteView) ByteSlice() []byte {
	c := make([]byte,b.Len())
	copy(c,b.bt)
	return c
}

func (b ByteView) String() string {
	return string(b.bt)
}
