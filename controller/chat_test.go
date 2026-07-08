package controller

import (
	"context"
	"testing"

	"go-alpha/models"
)

func TestNormalizeMessageBody(t *testing.T) {
	cases := []struct {
		name     string
		body     postMessageBody
		expected string
		msgType  int
	}{
		{
			name:     "image default content",
			body:     postMessageBody{MessageType: models.MsgImage},
			expected: "[图片]",
			msgType:  models.MsgImage,
		},
		{
			name:     "file default content",
			body:     postMessageBody{MessageType: models.MsgFile},
			expected: "[文件]",
			msgType:  models.MsgFile,
		},
		{
			name:     "emoji default content",
			body:     postMessageBody{MessageType: models.MsgEmoji},
			expected: "[表情]",
			msgType:  models.MsgEmoji,
		},
		{
			name:     "text default type",
			body:     postMessageBody{},
			expected: "",
			msgType:  models.MsgText,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalizeMessageBody(&tc.body)
			if tc.body.Content != tc.expected {
				t.Fatalf("content = %q, want %q", tc.body.Content, tc.expected)
			}
			if tc.body.MessageType != tc.msgType {
				t.Fatalf("message type = %d, want %d", tc.body.MessageType, tc.msgType)
			}
		})
	}
}

func TestPrivateConversationIDRoundTrip(t *testing.T) {
	convID := models.PrivateConvID(42, 7)
	if convID != "p_7_42" {
		t.Fatalf("convID = %q, want %q", convID, "p_7_42")
	}

	if got := models.PrivateConvUserA(convID); got != 7 {
		t.Fatalf("userA = %d, want 7", got)
	}
	if got := models.PrivateConvUserB(convID); got != 42 {
		t.Fatalf("userB = %d, want 42", got)
	}
	if got := models.PrivateConvRecipient(7, convID); got != 42 {
		t.Fatalf("recipient from 7 = %d, want 42", got)
	}
	if got := models.PrivateConvRecipient(42, convID); got != 7 {
		t.Fatalf("recipient from 42 = %d, want 7", got)
	}
}

func TestPrivateConversationLegacyCompatibility(t *testing.T) {
	legacyConvID := "1048586"
	if got := models.PrivateConvUserA(legacyConvID); got == 0 {
		t.Fatalf("legacy userA should parse")
	}
	if got := models.PrivateConvUserB(legacyConvID); got == 0 {
		t.Fatalf("legacy userB should parse")
	}
}

func TestSuccessMessagePayload(t *testing.T) {
	msg := models.Message{
		ID:             11,
		ConversationID: "p_1_2",
		ChatType:       "private",
		SenderID:       1,
		ReceiverID:     2,
		MessageType:    models.MsgText,
		Content:        "hello",
		FileName:       "demo.png",
		FileURL:        "https://example.com/file.png",
		Status:         1,
	}
	sender := models.User{Username: "alice", Avatar: "/a.png"}
	payload := successMessagePayload(msg, sender)

	if payload["conversation_id"] != "p_1_2" {
		t.Fatalf("conversation_id = %v, want p_1_2", payload["conversation_id"])
	}
	if payload["chat_type"] != "private" {
		t.Fatalf("chat_type = %v, want private", payload["chat_type"])
	}
	if payload["receiver_id"] != uint(2) {
		t.Fatalf("receiver_id = %v, want 2", payload["receiver_id"])
	}
	if payload["file_url"] != "https://example.com/file.png" {
		t.Fatalf("file_url = %v, want file url", payload["file_url"])
	}
	if payload["file_name"] != "demo.png" {
		t.Fatalf("file_name = %v, want demo.png", payload["file_name"])
	}
}

func BenchmarkNormalizeMessageBody(b *testing.B) {
	body := postMessageBody{MessageType: models.MsgImage}
	for i := 0; i < b.N; i++ {
		tmp := body
		normalizeMessageBody(&tmp)
	}
}

func BenchmarkSuccessMessagePayload(b *testing.B) {
	msg := models.Message{
		ID:             11,
		ConversationID: "p_1_2",
		ChatType:       "private",
		SenderID:       1,
		ReceiverID:     2,
		MessageType:    models.MsgText,
		Content:        "hello",
		Status:         1,
	}
	sender := models.User{Username: "alice", Avatar: "/a.png"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = successMessagePayload(msg, sender)
	}
}

func BenchmarkOnlineUserSet(b *testing.B) {
	hub := NewHub()
	for i := 0; i < 2000; i++ {
		hub.clients[&Client{userID: uint(i % 500)}] = true
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hub.OnlineUserSet()
	}
}

func TestChatUserContactsCacheSupportsEmptyList(t *testing.T) {
	if models.RDB == nil {
		t.Skip("Redis is not configured in test environment")
	}
	userID := uint(999999)
	contacts := []ContactInfo{}
	setCachedChatUserContacts(userID, contacts)
	defer func() {
		if models.RDB != nil {
			_ = models.RDB.Del(context.Background(), chatUserContactsCacheKey(userID)).Err()
		}
	}()

	got, ok := getCachedChatUserContacts(userID)
	if !ok {
		t.Fatalf("expected empty contacts cache to be readable")
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}
