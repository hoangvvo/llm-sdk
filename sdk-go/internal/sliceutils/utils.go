package sliceutils

func Map[T any, U any](input []T, mapper func(T) U) []U {
	output := make([]U, len(input))
	for i, v := range input {
		output[i] = mapper(v)
	}
	return output
}

func MapErr[T any, U any](input []T, mapper func(T) (U, error)) ([]U, error) {
	output := make([]U, len(input))
	for i, v := range input {
		mapped, err := mapper(v)
		if err != nil {
			return nil, err
		}
		output[i] = mapped
	}
	return output, nil
}

func Flat[T any](input [][]T) []T {
	var output []T
	for _, arr := range input {
		output = append(output, arr...)
	}
	return output
}
