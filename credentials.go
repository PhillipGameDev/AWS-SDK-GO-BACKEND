package awsbase

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

func getCredentialsProvider(ctx context.Context, c *Config) (aws.CredentialsProvider, string, error) {
	loadOptions, err := commonLoadOptions(c)
	if err != nil {
		return nil, "", err
	}
	loadOptions = append(
		loadOptions,
		// Bypass retries when validating authentication
		config.WithRetryer(func() aws.Retryer {
			return aws.NopRetryer{}
		}),
		// The endpoint resolver is added here instead of in commonLoadOptions() so that it
		// is not included in the aws.Config returned to the caller
		config.WithEndpointResolverWithOptions(credentialsEndpointResolver(c)),
	)

	envConfig, err := config.NewEnvConfig()
	if err != nil {
		return nil, "", err
	}

	profile := c.Profile
	if profile == "" {
		profile = envConfig.SharedConfigProfile
	}

	sharedCredentialsFiles, err := c.ResolveSharedCredentialsFiles()
	if err != nil {
		return nil, "", err
	}
	if len(sharedCredentialsFiles) == 0 {
		sharedCredentialsFiles = []string{envConfig.SharedCredentialsFile}
	}

	sharedConfigFiles, err := c.ResolveSharedConfigFiles()
	if err != nil {
		return nil, "", err
	}
	if len(sharedConfigFiles) == 0 {
		sharedConfigFiles = []string{envConfig.SharedConfigFile}
	}

	// The default AWS SDK authentication flow silently ignores invalid Profiles. Pre-validate that the Profile exists
	// https://github.com/aws/aws-sdk-go-v2/issues/1591
	if profile != "" {
		_, err := config.LoadSharedConfigProfile(ctx, profile, func(opts *config.LoadSharedConfigOptions) {
			opts.CredentialsFiles = sharedCredentialsFiles
			opts.ConfigFiles = sharedConfigFiles
		})
		if err != nil {
			return nil, "", err
		}
		loadOptions = append(
			loadOptions,
			config.WithSharedConfigProfile(c.Profile),
		)

	}

	if c.AccessKey != "" || c.SecretKey != "" || c.Token != "" {
		loadOptions = append(
			loadOptions,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					c.AccessKey,
					c.SecretKey,
					c.Token,
				),
			),
		)
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, "", fmt.Errorf("loading configuration: %w", err)
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, "", c.NewNoValidCredentialSourcesError(err)
	}

	if c.AssumeRole == nil || c.AssumeRole.RoleARN == "" {
		return cfg.Credentials, creds.Source, nil
	}

	provider, err := assumeRoleCredentialsProvider(ctx, cfg, c)

	return provider, creds.Source, err
}

func assumeRoleCredentialsProvider(ctx context.Context, awsConfig aws.Config, c *Config) (aws.CredentialsProvider, error) {
	ar := c.AssumeRole
	// When assuming a role, we need to first authenticate the base credentials above, then assume the desired role
	log.Printf("[INFO] Attempting to AssumeRole %s (SessionName: %q, ExternalId: %q)",
		ar.RoleARN, ar.SessionName, ar.ExternalID)

	client := stsClient(awsConfig, c)

	appCreds := stscreds.NewAssumeRoleProvider(client, ar.RoleARN, func(opts *stscreds.AssumeRoleOptions) {
		opts.RoleSessionName = ar.SessionName
		opts.Duration = ar.Duration

		if ar.ExternalID != "" {
			opts.ExternalID = aws.String(ar.ExternalID)
		}

		if ar.Policy != "" {
			opts.Policy = aws.String(ar.Policy)
		}

		if len(ar.PolicyARNs) > 0 {
			var policyDescriptorTypes []types.PolicyDescriptorType

			for _, policyARN := range ar.PolicyARNs {
				policyDescriptorType := types.PolicyDescriptorType{
					Arn: aws.String(policyARN),
				}
				policyDescriptorTypes = append(policyDescriptorTypes, policyDescriptorType)
			}

			opts.PolicyARNs = policyDescriptorTypes
		}

		if len(ar.Tags) > 0 {
			var tags []types.Tag
			for k, v := range ar.Tags {
				tag := types.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				}
				tags = append(tags, tag)
			}

			opts.Tags = tags
		}

		if len(ar.TransitiveTagKeys) > 0 {
			opts.TransitiveTagKeys = ar.TransitiveTagKeys
		}
	})
	_, err := appCreds.Retrieve(ctx)
	if err != nil {
		return nil, c.NewCannotAssumeRoleError(err)
	}
	return aws.NewCredentialsCache(appCreds), nil
}
