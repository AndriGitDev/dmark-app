package web

import "strings"

type FlashKind string

const (
	FlashSuccess FlashKind = "success"
	FlashError   FlashKind = "error"
	FlashInfo    FlashKind = "info"
)

type FlashMessage struct {
	Kind    FlashKind
	Message string
}

func NewFlash(kind FlashKind, message string) FlashMessage {
	message = strings.TrimSpace(message)
	if kind == "" {
		kind = FlashInfo
	}
	return FlashMessage{Kind: kind, Message: message}
}

func DomainAddedFlash(domain string) FlashMessage {
	return NewFlash(FlashSuccess, "Saved "+strings.TrimSpace(domain)+".")
}

func DomainDeletedFlash(domain string) FlashMessage {
	return NewFlash(FlashSuccess, "Deleted "+strings.TrimSpace(domain)+".")
}

func DomainRecheckQueuedFlash(domain string) FlashMessage {
	return NewFlash(FlashInfo, "Recheck queued for "+strings.TrimSpace(domain)+".")
}

func ErrorFlash(message string) FlashMessage {
	return NewFlash(FlashError, message)
}

func VisibleFlashes(messages []FlashMessage) []FlashMessage {
	visible := []FlashMessage{}
	for _, message := range messages {
		message.Message = strings.TrimSpace(message.Message)
		if message.Message == "" {
			continue
		}
		if message.Kind == "" {
			message.Kind = FlashInfo
		}
		visible = append(visible, message)
	}
	return visible
}
