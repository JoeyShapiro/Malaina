package internal

func Contains[T any](slice []T, value T, comp func(T, T) bool) bool {
	for _, item := range slice {
		if comp(item, value) {
			return true
		}
	}
	return false
}
