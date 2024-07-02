package main

import "context"

type Repository interface {
	GetTags(context.Context, string) ([]string, error)
}

// QuayRepository implements Quay Repository access
type QuayRepository struct {
	QuayURL     string
	PullSecret  string
	BearerToken string
}

// GetTags returns the tags for the given image
func (q *QuayRepository) GetTags(ctx context.Context, image string) ([]string, error) {
	return nil, nil
}

// ACRRepository implements ACR Repository access
type ACRRepository struct {
	ACRName  string
	MSIToken string
}

// GetTags returns the tags for the given image
func (a *ACRRepository) GetTags(ctx context.Context, image string) ([]string, error) {
	return nil, nil
}
