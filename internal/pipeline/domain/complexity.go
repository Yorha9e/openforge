package domain

func ClassifyComplexity(files, modules int) string {
	switch {
	case files <= 1 && modules <= 1:
		return "L1"
	case files <= 5 && modules <= 3:
		return "L2"
	case files < 10 && modules < 5:
		return "L3"
	default:
		return "L4"
	}
}
