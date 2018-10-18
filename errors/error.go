package errors

import "strings"

type ErrorCollection []error

func (ec *ErrorCollection) Add(e error) {
	*ec = append(*ec, e)
}

func (ec *ErrorCollection) Error() string {
	var errStrs []string
	for _, err := range *ec {
		errStrs = append(errStrs, err.Error())
	}
	return strings.Join(errStrs, "\n")
}
