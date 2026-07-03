package mail

import "testing"

func TestFakeMailer_CapturesSend(t *testing.T) {
	f := NewFake()
	err := f.Send("to@x.com", "hi", "body")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Messages) != 1 || f.Messages[0].To != "to@x.com" {
		t.Errorf("captured %+v", f.Messages)
	}
}
