package persona

// ValidINN accepts Russian 10- and 12-digit INNs with correct check digits.
func ValidINN(value string) bool {
	if len(value) != 10 && len(value) != 12 {
		return false
	}
	digits := make([]int, len(value))
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
		digits[i] = int(value[i] - '0')
	}
	if len(digits) == 10 {
		return checksum(digits, []int{2, 4, 10, 3, 5, 9, 4, 6, 8}) == digits[9]
	}
	return checksum(digits, []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8, 0}) == digits[10] &&
		checksum(digits, []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8, 0}) == digits[11]
}

func checksum(digits, weights []int) int {
	sum := 0
	for i, weight := range weights {
		sum += digits[i] * weight
	}
	return sum % 11 % 10
}
