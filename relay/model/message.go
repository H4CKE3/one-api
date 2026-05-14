package model

import (
	"fmt"
	"strings"
)

type Message struct {
	Role             string  `json:"role,omitempty"`
	Content          any     `json:"content,omitempty"`
	ReasoningContent any     `json:"reasoning_content,omitempty"`
	Name             *string `json:"name,omitempty"`
	ToolCalls        []Tool  `json:"tool_calls,omitempty"`
	ToolCallId       string  `json:"tool_call_id,omitempty"`
}

func (m Message) IsStringContent() bool {
	_, ok := m.Content.(string)
	return ok
}

func (m Message) StringContent() string {
	content, ok := m.Content.(string)
	if ok {
		return content
	}
	contentList, ok := m.Content.([]any)
	if ok {
		var contentStr string
		for _, contentItem := range contentList {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			if contentMap["type"] == ContentTypeText {
				if subStr, ok := contentMap["text"].(string); ok {
					contentStr += subStr
				}
			}
		}
		return contentStr
	}
	return ""
}

func (m Message) ParseContent() []MessageContent {
	var contentList []MessageContent
	content, ok := m.Content.(string)
	if ok {
		contentList = append(contentList, MessageContent{
			Type: ContentTypeText,
			Text: content,
		})
		return contentList
	}
	anyList, ok := m.Content.([]any)
	if ok {
		for _, contentItem := range anyList {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			switch contentMap["type"] {
			case ContentTypeText:
				if subStr, ok := contentMap["text"].(string); ok {
					contentList = append(contentList, MessageContent{
						Type: ContentTypeText,
						Text: subStr,
					})
				}
			case ContentTypeImageURL:
				if subObj, ok := contentMap["image_url"].(map[string]any); ok {
					url, ok := subObj["url"].(string)
					if !ok {
						continue
					}
					contentList = append(contentList, MessageContent{
						Type: ContentTypeImageURL,
						ImageURL: &ImageURL{
							Url: url,
						},
					})
				}
			case ContentTypeFile:
				if subObj, ok := contentMap["file"].(map[string]any); ok {
					contentList = append(contentList, MessageContent{
						Type: ContentTypeFile,
						File: parseFilePayload(subObj),
					})
				}
			}
		}
		return contentList
	}
	return nil
}

func parseFilePayload(payload map[string]any) *FilePayload {
	file := &FilePayload{}
	if filename, ok := payload["filename"].(string); ok {
		file.Filename = filename
	}
	if fileData, ok := payload["file_data"].(string); ok {
		file.FileData = fileData
	}
	if fileID, ok := payload["file_id"].(string); ok {
		file.FileID = fileID
	}
	return file
}

func ParseFileDataURI(fileData string) (mimeType string, data string, err error) {
	if !strings.HasPrefix(fileData, "data:") {
		return "", "", fmt.Errorf("file_data must be a data URI")
	}
	comma := strings.Index(fileData, ",")
	if comma < 0 {
		return "", "", fmt.Errorf("invalid file_data URI")
	}

	meta := fileData[len("data:"):comma]
	if !strings.HasSuffix(meta, ";base64") {
		return "", "", fmt.Errorf("file_data must be base64 encoded")
	}

	mimeType = strings.TrimSuffix(meta, ";base64")
	if mimeType == "" {
		return "", "", fmt.Errorf("file_data MIME type is required")
	}
	data = fileData[comma+1:]
	if data == "" {
		return "", "", fmt.Errorf("file_data base64 content is required")
	}
	return mimeType, data, nil
}

type ImageURL struct {
	Url    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type FilePayload struct {
	Filename string `json:"filename,omitempty"`
	FileData string `json:"file_data,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type MessageContent struct {
	Type     string       `json:"type,omitempty"`
	Text     string       `json:"text"`
	ImageURL *ImageURL    `json:"image_url,omitempty"`
	File     *FilePayload `json:"file,omitempty"`
}
