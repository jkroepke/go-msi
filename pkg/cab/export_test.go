package cab

// WithLZXBlock returns opts with every LZX block forced to one type:
// verbatim (1) or aligned offset (2).
func WithLZXBlock(opts Options, block int) Options {
	opts.lzxBlock = block

	return opts
}
