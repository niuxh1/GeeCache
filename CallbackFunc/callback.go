package callbackfunc

type CallbackFunc func(key string) ([]byte ,error)

func (f CallbackFunc) Get(key string)([]byte ,error)  {
	return f(key)
}
