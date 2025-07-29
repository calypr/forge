package fence

import (
	"strings"
)

type PingResp struct {
	Profile        string            `yaml:"profile"`
	Username       string            `yaml:"username"`
	Endpoint       string            `yaml:"endpoint"`
	BucketPrograms map[string]string `yaml:"bucket_programs"` // Assuming this is a slice of strings
	YourAccess     map[string]string `yaml:"your_access"`     // Assuming this is a slice of strings
}

type UserPermission struct {
	Method  string `json:"method"`
	Service string `json:"service"`
}

type FenceUserResp struct {
	Active                      bool                        `json:"active"`
	Authz                       map[string][]UserPermission `json:"authz"` // Map to handle dynamic keys like "/data_file"
	Azp                         *string                     `json:"azp"`   // Can be null, so use a pointer
	CertificatesUploaded        []any                       `json:"certificates_uploaded"`
	DisplayName                 string                      `json:"display_name"`
	Email                       string                      `json:"email"`
	Ga4GhPassportV1             []any                       `json:"ga4gh_passport_v1"`
	Groups                      []any                       `json:"groups"`
	Idp                         string                      `json:"idp"`
	IsAdmin                     bool                        `json:"is_admin"`
	Message                     string                      `json:"message"`
	Name                        string                      `json:"name"`
	PhoneNumber                 string                      `json:"phone_number"`
	PreferredUsername           string                      `json:"preferred_username"`
	PrimaryGoogleServiceAccount *string                     `json:"primary_google_service_account"` // Can be null
	ProjectAccess               map[string]any              `json:"project_access"`                 // Empty object, use map[string]interface{} for flexibility
	Resources                   []string                    `json:"resources"`
	ResourcesGranted            []any                       `json:"resources_granted"`
	Role                        string                      `json:"role"`
	Sub                         string                      `json:"sub"`
	UserID                      int                         `json:"user_id"`
	Username                    string                      `json:"username"`
}

type BucketResp struct {
	GSBuckets map[string]any        `json:"GS_BUCKETS"`
	S3Buckets map[string]BucketInfo `json:"S3_BUCKETS"`
}
type BucketInfo struct {
	EndpointUrl string   `json:"endpoint_url"`
	Programs    []string `json:"programs"`
	Region      string   `json:"region"`
}

func ParseBucketResp(resp BucketResp) map[string]string {
	bucketsByProgram := make(map[string]string)
	for bucketName, BucketInfo := range resp.S3Buckets {
		var programs strings.Builder
		if len(BucketInfo.Programs) > 1 {
			for _, p := range BucketInfo.Programs {
				programs.WriteString(p + ",")
			}
		} else if len(BucketInfo.Programs) == 1 {
			programs.WriteString(BucketInfo.Programs[0])
		}
		bucketsByProgram[bucketName] = programs.String()
	}
	return bucketsByProgram
}

func ParseUserResp(resp FenceUserResp) map[string]string {
	servicesByPath := make(map[string]string)
	for path, permissions := range resp.Authz {
		var services strings.Builder
		seenServices := make(map[string]bool)
		for _, p := range permissions {
			if !seenServices[p.Method] {
				services.WriteString(p.Method + ",")
				seenServices[p.Method] = true
			}
		}
		servicesByPath[path] = services.String()[:services.Len()-1]
	}
	return servicesByPath
}
