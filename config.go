package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/viper"
)

type Config struct {
	Region  string
	Cognito CognitoConfig
	Gateway APIGateway
	IoT     AWSIoT

	Installation Installation

	API API
}

type CognitoConfig struct {
	Username       string
	Password       string
	UserPoolID     string `mapstructure:"user_pool_id"`
	ClientID       string `mapstructure:"client_id"`
	IdentityPoolID string `mapstructure:"identity_pool_id"`
}

type APIGateway struct {
	Name string
}

type AWSIoT struct {
	Name string
}

type Installation struct {
	ID string
}

type API struct {
	Endpoint string
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("viper.ReadInConfig failed: %w", err)
	}

	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("viper.Unmarshal failed: %w", err)
	}
	return &c, nil
}

func loadAWSConfig(ctx context.Context, c *Config) (*aws.Config, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx,
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("awsconfig.LoadDefaultConfig failed: %w", err)
	}
	return &awsConfig, nil
}
