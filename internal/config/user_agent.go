package config

import (
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func (ua UserAgentProducts) BuildUserAgentString() string {
	builder := smithyhttp.NewUserAgentBuilder()
	for _, p := range ua {
		p.buildUserAgentPart(builder)
	}
	return builder.Build()
}

func (p UserAgentProduct) buildUserAgentPart(b *smithyhttp.UserAgentBuilder) {
	if p.Name != "" {
		if p.Version != "" {
			b.AddKeyValue(p.Name, p.Version)
		} else {
			b.AddKey(p.Name)
		}
	}
	if p.Comment != "" {
		b.AddKey("(" + p.Comment + ")")
	}
}
