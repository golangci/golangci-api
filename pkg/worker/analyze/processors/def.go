package processors

const (
	internalError = "Internal error"

	statusSentToQueue = "sent_to_queue"
	statusProcessing  = "processing"
	statusProcessed   = "processed"
	statusNotFound    = "not_found"

	noGoFilesToAnalyzeMessage = "No Go files to analyze"
	noGoFilesToAnalyzeErr     = "no go files to analyze"
)
