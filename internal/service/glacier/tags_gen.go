// Code generated by internal/generate/tags/main.go; DO NOT EDIT.
package glacier

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/aws/aws-sdk-go/service/glacier/glacieriface"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
)

// ListTags lists glacier service tags.
// The identifier is typically the Amazon Resource Name (ARN), although
// it may also be a different identifier depending on the service.
func ListTags(conn glacieriface.GlacierAPI, identifier string) (tftags.KeyValueTags, error) {
	return ListTagsWithContext(context.Background(), conn, identifier)
}

func ListTagsWithContext(ctx context.Context, conn glacieriface.GlacierAPI, identifier string) (tftags.KeyValueTags, error) {
	input := &glacier.ListTagsForVaultInput{
		VaultName: aws.String(identifier),
	}

	output, err := conn.ListTagsForVaultWithContext(ctx, input)

	if err != nil {
		return tftags.New(nil), err
	}

	return KeyValueTags(output.Tags), nil
}

// map[string]*string handling

// Tags returns glacier service tags.
func Tags(tags tftags.KeyValueTags) map[string]*string {
	return aws.StringMap(tags.Map())
}

// KeyValueTags creates KeyValueTags from glacier service tags.
func KeyValueTags(tags map[string]*string) tftags.KeyValueTags {
	return tftags.New(tags)
}

// UpdateTags updates glacier service tags.
// The identifier is typically the Amazon Resource Name (ARN), although
// it may also be a different identifier depending on the service.
func UpdateTags(conn glacieriface.GlacierAPI, identifier string, oldTags interface{}, newTags interface{}) error {
	return UpdateTagsWithContext(context.Background(), conn, identifier, oldTags, newTags)
}
func UpdateTagsWithContext(ctx context.Context, conn glacieriface.GlacierAPI, identifier string, oldTagsMap interface{}, newTagsMap interface{}) error {
	oldTags := tftags.New(oldTagsMap)
	newTags := tftags.New(newTagsMap)

	if removedTags := oldTags.Removed(newTags); len(removedTags) > 0 {
		input := &glacier.RemoveTagsFromVaultInput{
			VaultName: aws.String(identifier),
			TagKeys:   aws.StringSlice(removedTags.IgnoreAWS().Keys()),
		}

		_, err := conn.RemoveTagsFromVaultWithContext(ctx, input)

		if err != nil {
			return fmt.Errorf("untagging resource (%s): %w", identifier, err)
		}
	}

	if updatedTags := oldTags.Updated(newTags); len(updatedTags) > 0 {
		input := &glacier.AddTagsToVaultInput{
			VaultName: aws.String(identifier),
			Tags:      Tags(updatedTags.IgnoreAWS()),
		}

		_, err := conn.AddTagsToVaultWithContext(ctx, input)

		if err != nil {
			return fmt.Errorf("tagging resource (%s): %w", identifier, err)
		}
	}

	return nil
}
