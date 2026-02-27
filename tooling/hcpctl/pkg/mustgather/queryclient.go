// Copyright 2025 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mustgather

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"

	azkquery "github.com/Azure/azure-kusto-go/azkustodata/query"

	"github.com/Azure/ARO-HCP/tooling/hcpctl/pkg/kusto"
)

// TaggedRow pairs a raw Kusto row with the name of the query that produced it.
type TaggedRow struct {
	Row       azkquery.Row
	QueryName string
}

// QueryClientInterface defines the interface for querying data
type QueryClientInterface interface {
	ConcurrentQueries(ctx context.Context, queries []*kusto.ConfigurableQuery, outputChannel chan<- TaggedRow) error
	Close() error
	ExecutePreconfiguredQuery(ctx context.Context, query *kusto.ConfigurableQuery, outputChannel chan<- azkquery.Row) (*kusto.QueryResult, error)
}

type QueryClient struct {
	Client       kusto.KustoClient
	QueryTimeout time.Duration
	OutputPath   string
	FileWriter   FileWriter
}

// NewQueryClient creates a new QueryClient with default dependencies
func NewQueryClient(client kusto.KustoClient, queryTimeout time.Duration, outputPath string) *QueryClient {
	return &QueryClient{
		Client:       client,
		QueryTimeout: queryTimeout,
		OutputPath:   outputPath,
		FileWriter:   &JsonEncoderWriter{},
	}
}

// NewQueryClient creates a new QueryClient with default dependencies
func NewQueryClientWithFileWriter(client kusto.KustoClient, queryTimeout time.Duration, outputPath string, fileWriter FileWriter) *QueryClient {
	return &QueryClient{
		Client:       client,
		QueryTimeout: queryTimeout,
		OutputPath:   outputPath,
		FileWriter:   fileWriter,
	}
}

func (q *QueryClient) ConcurrentQueries(ctx context.Context, queries []*kusto.ConfigurableQuery, outputChannel chan<- TaggedRow) error {
	logger := logr.FromContextOrDiscard(ctx)

	queryGroup, queryCtx := errgroup.WithContext(ctx)
	for _, query := range queries {
		queryGroup.Go(func() error {
			rowChan := make(chan azkquery.Row)

			innerGroup, innerCtx := errgroup.WithContext(queryCtx)

			// Execute the query, writing raw rows to the intermediate channel.
			innerGroup.Go(func() error {
				defer close(rowChan)
				result, err := q.Client.ExecutePreconfiguredQuery(innerCtx, query, rowChan)
				if err != nil {
					logger.Error(err, "Query failed", "name", query.Name)
					return fmt.Errorf("failed to execute query: %w", err)
				}
				if q.FileWriter != nil {
					if err := q.FileWriter.WriteFile(q.OutputPath, fmt.Sprintf("%s.json", query.Name), result); err != nil {
						return fmt.Errorf("failed to write query result to file: %w", err)
					}
				}
				return nil
			})

			// Forward rows tagged with the query name.
			innerGroup.Go(func() error {
				for {
					select {
					case <-innerCtx.Done():
						return innerCtx.Err()
					case row, ok := <-rowChan:
						if !ok {
							return nil
						}
						select {
						case <-innerCtx.Done():
							return innerCtx.Err()
						case outputChannel <- TaggedRow{Row: row, QueryName: query.Name}:
						}
					}
				}
			})

			return innerGroup.Wait()
		})
	}

	return queryGroup.Wait()
}

func (q *QueryClient) Close() error {
	return q.Client.Close()
}

func (q *QueryClient) ExecutePreconfiguredQuery(ctx context.Context, query *kusto.ConfigurableQuery, outputChannel chan<- azkquery.Row) (*kusto.QueryResult, error) {
	return q.Client.ExecutePreconfiguredQuery(ctx, query, outputChannel)
}
