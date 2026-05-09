package report

import (
	"context"
	"strings"

	"github-report-ai/pkg/ai"
)

const SecurityAuditPrompt = `You are a security auditor. Analyze the following git commit log for leaked secrets like API keys, passwords, private keys, or sensitive credentials. 

Respond ONLY with a list of findings in this format:
- [COMMIT_HASH] [SECRET_TYPE]: [DESCRIPTION]

If no secrets are found, respond with "OK".

Commit Log:
`

func AuditSecurity(ctx context.Context, mod, key, data string) ([]string, error) {
	var res string
	var err error
	
	if strings.HasPrefix(mod, "gemini") {
		id := "gemini-2.5-flash"
		if mod != "gemini-flash" {
			id = "gemini-2.5-flash-lite"
		}
		res, _, err = ai.NewGeminiClient(key).GenerateReport(ctx, id, SecurityAuditPrompt, data)
	} else {
		id := "llama-3.1-8b-instant"
		if mod == "groq-mixtral" {
			id = "mixtral-8x7b-32768"
		}
		res, _, err = ai.NewGroqClient(key).GenerateReport(ctx, id, SecurityAuditPrompt, data)
	}

	if err != nil {
		return nil, err
	}

	res = strings.TrimSpace(res)
	if res == "OK" || res == "" {
		return nil, nil
	}

	lines := strings.Split(res, "\n")
	var findings []string
	for _, l := range lines {
		if strings.HasPrefix(l, "- ") {
			findings = append(findings, l)
		}
	}

	return findings, nil
}
