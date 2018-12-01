package errorutils

type InternalError struct {
	PublicDesc  string
	PrivateDesc string
}

func (e InternalError) Error() string {
	return e.PrivateDesc
}

type BadInputError struct {
	PublicDesc string
}

func (e BadInputError) Error() string {
	return e.PublicDesc
}
