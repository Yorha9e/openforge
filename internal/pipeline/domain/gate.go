package domain

type GateChecklist struct {
	CodeReviewed      bool
	SecurityChecked   bool
	LicenseCleared    bool
	CodingStandardMet bool
}

type LineComment struct {
	FilePath string
	Line     int
	Comment  string
	Mark     string
}

type GateEvent struct {
	PipelineID      string
	Stage           string
	Event           string
	Actor           string
	LineComments    []LineComment
	SummaryFeedback string
	Checklist       GateChecklist
}
