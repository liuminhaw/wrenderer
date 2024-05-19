package shared

import "strings"

type ResponseData struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	ObjectKey string `json:"objectKey"`
}

func (rd ResponseData) GetObjectPath() string {
	var objectPath string
	if rd.Port != "" {
		objectPath = strings.Join([]string{rd.Host, rd.Port}, "_")
		objectPath = strings.Join([]string{objectPath, rd.ObjectKey}, "/")
	} else {
		objectPath = strings.Join([]string{rd.Host, rd.ObjectKey}, "/")
	}

	return objectPath
}
