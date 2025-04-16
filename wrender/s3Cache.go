package wrender

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/smithy-go"
)

type S3CachingMeta struct {
	Bucket      string
	Region      string
	ContentType string
}

type S3Caching struct {
	Client       *s3.Client
	Meta         S3CachingMeta
	CachedPrefix string
	CachedPath   string
}

func NewS3Caching(
	client *s3.Client,
	prefix string,
	keyPath string,
	meta S3CachingMeta,
) S3Caching {
	return S3Caching{
		Client:       client,
		Meta:         meta,
		CachedPrefix: prefix,
		CachedPath:   keyPath,
	}
}

func (c S3Caching) Update(reader io.Reader) error {
	return c.putObject(c.CachedPath, reader)
}

func (c S3Caching) UpdateTo(reader io.Reader, suffixPath string) error {
	key := filepath.Join(c.CachedPrefix, suffixPath)
	return c.putObject(key, reader)
}

func (c S3Caching) putObject(key string, reader io.Reader) error {
	_, err := c.Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(c.Meta.Bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(c.Meta.ContentType),
	})
	return err
}

func (c S3Caching) Delete() error {
	_, err := c.Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(c.Meta.Bucket),
		Key:    aws.String(c.CachedPath),
	})
	return err
}

// DeletePrefix deletes all objects matching CachedPrefix value from S3Caching variable.
func (c S3Caching) DeletePrefix() error {
	var prefix string
	if c.CachedPrefix == "" {
		return fmt.Errorf("empty CachedPrefix")
	} else if !strings.HasSuffix(c.CachedPrefix, "/") {
		prefix = c.CachedPrefix + "/"
	} else {
		prefix = c.CachedPrefix
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.Meta.Bucket),
		Prefix: aws.String(prefix),
	}

	for {
		result, err := c.Client.ListObjectsV2(context.Background(), input)
		if err != nil {
			return err
		}

		if len(result.Contents) == 0 {
			// No object left
			break
		}

		// Prepare the objects to delete
		var objectsToDelete []types.ObjectIdentifier
		for _, object := range result.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: object.Key,
			})
		}

		// Perform the delete operation
		_, err = c.Client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
			Bucket: aws.String(c.Meta.Bucket),
			Delete: &types.Delete{
				Objects: objectsToDelete,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}

		// Check if there are more objects to delete
		if *result.IsTruncated {
			input.ContinuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	return nil
}

// Read method reads the object from S3 bucket and returns its content if exists.
// If the object does not exist or is empty, a CacheNotFoundError will be returned.
func (c S3Caching) Read() (CacheContent, error) {
	obj, err := c.Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(c.Meta.Bucket),
		Key:    aws.String(c.CachedPath),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			switch apiErr.ErrorCode() {
			case "NoSuchKey":
				return nil, &CacheNotFoundError{err}
			default:
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	defer obj.Body.Close()

	content, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return nil, &CacheNotFoundError{fmt.Errorf("empty cache content")}
	}

	return CacheContent(content), nil
}

// Exists checks if S3Caching data exists.
func (c S3Caching) Exists() (bool, error) {
	objStats, err := c.Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(c.Meta.Bucket),
		Key:    aws.String(c.CachedPath),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			switch apiErr.ErrorCode() {
			case "NotFound":
				return false, nil
			default:
				return false, err
			}
		} else {
			return false, err
		}
	}

	// Check object content length
	if *objStats.ContentLength == 0 {
		return false, nil
	}

	// Object exists
	return true, nil
}

// IsEmptyPrefix checks if the S3 bucket is empty under certain prefix path.
// If suffixPath is empty, it checks if the CachedPrefix is empty.
// If suffixPath is not empty, it checks if the CachedPrefix/{suffixPath} is empty.
func (c S3Caching) IsEmptyPrefix(suffixPath string) (bool, error) {
	var path string
	if suffixPath == "" {
		path = c.CachedPrefix
	} else {
		path = filepath.Join(c.CachedPrefix, suffixPath)
	}

	result, err := c.Client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.Meta.Bucket),
		Prefix:  aws.String(path),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}

	if len(result.Contents) == 0 {
		return true, nil
	}

	return false, nil
}

func (c S3Caching) List(suffixPath string) ([]CacheContentInfo, error) {
	var path string
	if suffixPath == "" {
		path = c.CachedPrefix
	} else {
		path = filepath.Join(c.CachedPrefix, suffixPath)
	}

	paginator := s3.NewListObjectsV2Paginator(c.Client, &s3.ListObjectsV2Input{
		Bucket:     aws.String(c.Meta.Bucket),
		Prefix:     aws.String(path),
		StartAfter: aws.String(path),
	})

	var contents []CacheContentInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, content := range page.Contents {
			obj, err := c.Client.GetObject(context.Background(), &s3.GetObjectInput{
				Bucket: aws.String(c.Meta.Bucket),
				Key:    content.Key,
			})
			if err != nil {
				return nil, err
			}
			defer obj.Body.Close()

			body, err := io.ReadAll(obj.Body)
			if err != nil {
				return nil, err
			}

			contents = append(contents, CacheContentInfo{
				Content: CacheContent(body),
				Path:    *content.Key,
			})
		}
	}

	return contents, nil
}
