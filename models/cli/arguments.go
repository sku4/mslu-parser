package cli

import "context"

type Arguments struct {
	Profile            string
	Login              string
	Password           string
	ZeitMode           string
	ZeitType           string
	SpiegelSuchbegriff string
	SpiegelZeitraum    int
	SpiegelInhalt      string
	SpiegelSegments    string
	Count              int
	Update             bool
}

type argsKey struct{}

func SetArgs(ctx context.Context, args Arguments) context.Context {
	return context.WithValue(ctx, argsKey{}, args)
}

func GetArgs(ctx context.Context) Arguments {
	contextUser, _ := ctx.Value(argsKey{}).(Arguments)

	return contextUser
}
