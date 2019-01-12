package processors

type PullProcessorFactory interface {
	BuildProcessor(ctx *PullContext) (PullProcessor, func(), error)
}
