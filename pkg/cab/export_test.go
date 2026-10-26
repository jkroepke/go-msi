package cab

// WithLZXBlock returns opts with every LZX block forced to one type:
// verbatim (1), aligned offset (2) or uncompressed (3).
func WithLZXBlock(opts Options, block int) Options {
	opts.lzxBlock = block

	return opts
}
