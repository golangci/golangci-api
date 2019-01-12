package processors

const (
	internalError = "Internal error"

	StatusSentToQueue = "sent_to_queue"
	StatusProcessing  = "processing"
	StatusProcessed   = "processed"
	StatusNotFound    = "not_found"
	StatusError       = "error"

	noGoFilesToAnalyzeMessage = "No Go files to analyze"
	noGoFilesToAnalyzeErr     = "no go files to analyze"

	stepUpdateStatusToProcessing = `set analysis status to "processing"`
)
