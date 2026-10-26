package staticassets

import "embed"

//go:embed *.html *.js *.css views/*
var Files embed.FS
