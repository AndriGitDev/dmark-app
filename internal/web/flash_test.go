package web

import "testing"

func TestVisibleFlashesNormalizesAndDropsEmpty(t *testing.T) {
	t.Parallel()

	flashes := VisibleFlashes([]FlashMessage{
		{Message: "  saved  "},
		{Kind: FlashError, Message: "  "},
		{Kind: FlashSuccess, Message: "done"},
	})

	if len(flashes) != 2 {
		t.Fatalf("len(flashes) = %d, want 2", len(flashes))
	}
	if flashes[0].Kind != FlashInfo || flashes[0].Message != "saved" {
		t.Fatalf("first flash = %+v, want normalized info", flashes[0])
	}
	if flashes[1].Kind != FlashSuccess || flashes[1].Message != "done" {
		t.Fatalf("second flash = %+v, want success", flashes[1])
	}
}

func TestDomainFlashHelpers(t *testing.T) {
	t.Parallel()

	if flash := DomainAddedFlash("example.com"); flash.Kind != FlashSuccess || flash.Message != "Saved example.com." {
		t.Fatalf("DomainAddedFlash = %+v", flash)
	}
	if flash := DomainRecheckQueuedFlash("example.com"); flash.Kind != FlashInfo || flash.Message != "Recheck queued for example.com." {
		t.Fatalf("DomainRecheckQueuedFlash = %+v", flash)
	}
	if flash := ErrorFlash("failed"); flash.Kind != FlashError || flash.Message != "failed" {
		t.Fatalf("ErrorFlash = %+v", flash)
	}
}
