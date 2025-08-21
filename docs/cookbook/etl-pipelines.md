# Recipe: ETL Pipelines

Build robust Extract-Transform-Load pipelines for data processing at scale.

## Problem

You need to build ETL pipelines that:
- Extract data from multiple sources
- Transform data with complex business logic
- Handle large datasets efficiently
- Provide error recovery and monitoring
- Support both batch and streaming modes

## Solution

Create composable ETL pipelines with pipz:

```go
// ETL data structure
type ETLRecord struct {
    ID         string                 `json:"id"`
    Source     string                 `json:"source"`
    RawData    interface{}            `json:"raw_data"`
    Parsed     map[string]interface{} `json:"parsed"`
    Enriched   map[string]interface{} `json:"enriched"`
    Errors     []string               `json:"errors,omitempty"`
    Timestamp  time.Time              `json:"timestamp"`
}

// Implement Cloner for parallel processing
func (r ETLRecord) Clone() ETLRecord {
    // Deep copy maps
    parsed := make(map[string]interface{}, len(r.Parsed))
    for k, v := range r.Parsed {
        parsed[k] = v
    }
    
    enriched := make(map[string]interface{}, len(r.Enriched))
    for k, v := range r.Enriched {
        enriched[k] = v
    }
    
    errors := make([]string, len(r.Errors))
    copy(errors, r.Errors)
    
    return ETLRecord{
        ID:        r.ID,
        Source:    r.Source,
        RawData:   r.RawData,
        Parsed:    parsed,
        Enriched:  enriched,
        Errors:    errors,
        Timestamp: r.Timestamp,
    }
}

// Complete ETL pipeline
func createETLPipeline(config ETLConfig) pipz.Chainable[ETLRecord] {
    return pipz.NewSequence[ETLRecord]("etl-pipeline",
        // EXTRACT Phase
        pipz.Switch[ETLRecord]("extract",
            func(ctx context.Context, r ETLRecord) string {
                return r.Source
            },
        ).
        AddRoute("database", extractFromDatabase).
        AddRoute("api", extractFromAPI).
        AddRoute("file", extractFromFile).
        AddRoute("stream", extractFromStream),
        
        // TRANSFORM Phase
        pipz.NewSequence[ETLRecord]("transform",
            pipz.Apply("parse", parseRawData),
            pipz.Apply("validate", validateData),
            pipz.Transform("normalize", normalizeData),
            pipz.Apply("business-logic", applyBusinessRules),
            pipz.Enrich("external-data", enrichWithExternalData),
        ),
        
        // LOAD Phase
        pipz.NewConcurrent[ETLRecord]("load",
            pipz.Apply("primary-db", loadToPrimaryDB),
            pipz.Effect("cache", updateCache),
            pipz.Effect("search", indexInElasticsearch),
            pipz.Scaffold("analytics",  // Fire-and-forget
                pipz.Effect("data-lake", sendToDataLake),
                pipz.Effect("metrics", updateMetrics),
            ),
        ),
    )
}
```

## Extract Patterns

### Multi-Source Extraction

```go
// Database extraction with pagination
var extractFromDatabase = pipz.Apply("db-extract", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    query := r.RawData.(string)
    
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return r, fmt.Errorf("database query failed: %w", err)
    }
    defer rows.Close()
    
    var data []map[string]interface{}
    for rows.Next() {
        // Convert row to map
        record := scanToMap(rows)
        data = append(data, record)
    }
    
    r.RawData = data
    r.Parsed["count"] = len(data)
    return r, nil
})

// API extraction with retry
var extractFromAPI = pipz.NewRetry("api-extract",
    pipz.Apply("fetch", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
        endpoint := r.RawData.(string)
        
        resp, err := httpClient.Get(ctx, endpoint)
        if err != nil {
            return r, fmt.Errorf("API request failed: %w", err)
        }
        
        var data interface{}
        if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
            return r, fmt.Errorf("JSON decode failed: %w", err)
        }
        
        r.RawData = data
        return r, nil
    }),
    3, // Retry up to 3 times
)

// File extraction with streaming
var extractFromFile = pipz.Apply("file-extract", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    filepath := r.RawData.(string)
    
    file, err := os.Open(filepath)
    if err != nil {
        return r, fmt.Errorf("file open failed: %w", err)
    }
    defer file.Close()
    
    // Stream processing for large files
    scanner := bufio.NewScanner(file)
    var records []interface{}
    
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return r, ctx.Err()
        default:
            record := parseCSVLine(scanner.Text())
            records = append(records, record)
        }
    }
    
    r.RawData = records
    return r, nil
})
```

### Change Data Capture (CDC)

```go
type CDCExtractor struct {
    lastSync time.Time
    mu       sync.RWMutex
}

func (c *CDCExtractor) Extract(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    c.mu.RLock()
    since := c.lastSync
    c.mu.RUnlock()
    
    // Query only changed records
    query := fmt.Sprintf(`
        SELECT * FROM %s 
        WHERE updated_at > ? 
        ORDER BY updated_at ASC
    `, r.Parsed["table"])
    
    rows, err := db.QueryContext(ctx, query, since)
    if err != nil {
        return r, err
    }
    defer rows.Close()
    
    var changes []Change
    for rows.Next() {
        change := scanToChange(rows)
        changes = append(changes, change)
    }
    
    if len(changes) > 0 {
        c.mu.Lock()
        c.lastSync = changes[len(changes)-1].UpdatedAt
        c.mu.Unlock()
    }
    
    r.RawData = changes
    r.Parsed["change_count"] = len(changes)
    r.Parsed["sync_time"] = time.Now()
    
    return r, nil
}
```

## Transform Patterns

### Data Parsing and Validation

```go
var parseRawData = pipz.Apply("parse", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    switch data := r.RawData.(type) {
    case []byte:
        // Parse JSON
        var parsed map[string]interface{}
        if err := json.Unmarshal(data, &parsed); err != nil {
            r.Errors = append(r.Errors, fmt.Sprintf("JSON parse error: %v", err))
            return r, err
        }
        r.Parsed = parsed
        
    case string:
        // Parse CSV
        reader := csv.NewReader(strings.NewReader(data))
        records, err := reader.ReadAll()
        if err != nil {
            r.Errors = append(r.Errors, fmt.Sprintf("CSV parse error: %v", err))
            return r, err
        }
        r.Parsed["records"] = records
        
    case []map[string]interface{}:
        // Already parsed
        r.Parsed["data"] = data
        
    default:
        return r, fmt.Errorf("unsupported data type: %T", data)
    }
    
    return r, nil
})

var validateData = pipz.Apply("validate", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    // Required fields validation
    required := []string{"id", "timestamp", "type"}
    for _, field := range required {
        if _, ok := r.Parsed[field]; !ok {
            err := fmt.Errorf("missing required field: %s", field)
            r.Errors = append(r.Errors, err.Error())
            return r, err
        }
    }
    
    // Data type validation
    if id, ok := r.Parsed["id"].(string); ok {
        if !isValidUUID(id) {
            err := fmt.Errorf("invalid ID format: %s", id)
            r.Errors = append(r.Errors, err.Error())
            return r, err
        }
    }
    
    return r, nil
})
```

### Data Transformation

```go
var normalizeData = pipz.Transform("normalize", func(ctx context.Context, r ETLRecord) ETLRecord {
    // Standardize field names
    fieldMappings := map[string]string{
        "user_id":    "userId",
        "created_at": "createdAt",
        "updated_at": "updatedAt",
    }
    
    normalized := make(map[string]interface{})
    for old, new := range fieldMappings {
        if val, ok := r.Parsed[old]; ok {
            normalized[new] = val
            delete(r.Parsed, old)
        }
    }
    
    // Merge normalized fields
    for k, v := range normalized {
        r.Parsed[k] = v
    }
    
    // Standardize timestamps
    if ts, ok := r.Parsed["timestamp"].(string); ok {
        if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
            r.Parsed["timestamp"] = parsed
        }
    }
    
    return r
})

var applyBusinessRules = pipz.Apply("business-rules", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    // Calculate derived fields
    if price, ok := r.Parsed["price"].(float64); ok {
        if quantity, ok := r.Parsed["quantity"].(float64); ok {
            r.Parsed["total"] = price * quantity
        }
    }
    
    // Apply categorization
    if category := categorizeRecord(r.Parsed); category != "" {
        r.Parsed["category"] = category
    }
    
    // Data quality scoring
    qualityScore := calculateQualityScore(r.Parsed)
    r.Parsed["quality_score"] = qualityScore
    
    if qualityScore < 0.5 {
        r.Errors = append(r.Errors, "low quality score")
    }
    
    return r, nil
})
```

### Data Enrichment

```go
var enrichWithExternalData = pipz.Enrich("enrich", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    // Parallel enrichment from multiple sources
    enrichers := []func(context.Context, ETLRecord) (ETLRecord, error){
        enrichWithGeoData,
        enrichWithCustomerData,
        enrichWithMarketData,
    }
    
    for _, enricher := range enrichers {
        var err error
        r, err = enricher(ctx, r)
        if err != nil {
            // Log but don't fail
            log.Printf("Enrichment failed: %v", err)
            r.Errors = append(r.Errors, err.Error())
        }
    }
    
    return r, nil
})
```

## Load Patterns

### Multi-Destination Loading

```go
var loadToPrimaryDB = pipz.Apply("db-load", func(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return r, err
    }
    defer tx.Rollback()
    
    // Upsert logic
    query := `
        INSERT INTO etl_records (id, data, enriched, timestamp)
        VALUES (?, ?, ?, ?)
        ON CONFLICT (id) DO UPDATE SET
            data = EXCLUDED.data,
            enriched = EXCLUDED.enriched,
            timestamp = EXCLUDED.timestamp
    `
    
    dataJSON, _ := json.Marshal(r.Parsed)
    enrichedJSON, _ := json.Marshal(r.Enriched)
    
    if _, err := tx.ExecContext(ctx, query, 
        r.ID, dataJSON, enrichedJSON, r.Timestamp); err != nil {
        return r, err
    }
    
    return r, tx.Commit()
})

var indexInElasticsearch = pipz.Effect("elastic", func(ctx context.Context, r ETLRecord) error {
    doc := map[string]interface{}{
        "id":        r.ID,
        "source":    r.Source,
        "data":      r.Parsed,
        "enriched":  r.Enriched,
        "timestamp": r.Timestamp,
    }
    
    _, err := elasticClient.Index().
        Index("etl-records").
        Id(r.ID).
        BodyJson(doc).
        Do(ctx)
    
    return err
})
```

## Batch Processing

```go
// Batch processor for efficiency
type BatchProcessor struct {
    batchSize int
    timeout   time.Duration
    processor pipz.Chainable[[]ETLRecord]
    batch     []ETLRecord
    mu        sync.Mutex
}

func (b *BatchProcessor) Process(ctx context.Context, r ETLRecord) (ETLRecord, error) {
    b.mu.Lock()
    b.batch = append(b.batch, r)
    shouldProcess := len(b.batch) >= b.batchSize
    b.mu.Unlock()
    
    if shouldProcess {
        if err := b.processBatch(ctx); err != nil {
            return r, err
        }
    }
    
    return r, nil
}

func (b *BatchProcessor) processBatch(ctx context.Context) error {
    b.mu.Lock()
    if len(b.batch) == 0 {
        b.mu.Unlock()
        return nil
    }
    
    batch := b.batch
    b.batch = make([]ETLRecord, 0, b.batchSize)
    b.mu.Unlock()
    
    _, err := b.processor.Process(ctx, batch)
    return err
}

// Use batch processing in pipeline
batchETL := pipz.NewSequence[ETLRecord]("batch-etl",
    extractPhase,
    transformPhase,
    pipz.Apply("batch", batchProcessor.Process),
)
```

## Error Recovery

```go
// ETL with comprehensive error handling
resilientETL := pipz.NewHandle("etl-with-recovery",
    createETLPipeline(config),
    pipz.NewSequence[*pipz.Error[ETLRecord]]("error-recovery",
        pipz.Apply("classify", func(ctx context.Context, err *pipz.Error[ETLRecord]) (*pipz.Error[ETLRecord], error) {
            // Classify error type
            if isTransient(err.Cause) {
                // Retry transient errors
                return err, retryQueue.Add(err.State)
            }
            
            if isDataQuality(err.Cause) {
                // Send to data quality queue
                return err, qualityQueue.Add(err.State)
            }
            
            // Send to dead letter queue
            return err, dlq.Add(err.State)
        }),
        
        pipz.Effect("alert", func(ctx context.Context, err *pipz.Error[ETLRecord]) error {
            // Alert on critical errors
            if isCritical(err.Cause) {
                alerting.SendCritical("ETL Pipeline Failure", err)
            }
            return nil
        }),
    ),
)
```

## Monitoring and Metrics

```go
// ETL metrics collection
var etlMetrics = pipz.NewSequence[ETLRecord]("metrics",
    pipz.Effect("record-metrics", func(ctx context.Context, r ETLRecord) error {
        labels := map[string]string{
            "source": r.Source,
            "status": getStatus(r),
        }
        
        // Record processing metrics
        metrics.Increment("etl.records.processed", labels)
        
        // Record data quality
        if score, ok := r.Parsed["quality_score"].(float64); ok {
            metrics.RecordHistogram("etl.quality.score", score, labels)
        }
        
        // Record errors
        if len(r.Errors) > 0 {
            metrics.Increment("etl.errors", labels)
        }
        
        // Record processing time
        processingTime := time.Since(r.Timestamp)
        metrics.RecordHistogram("etl.processing.time_ms", 
            processingTime.Milliseconds(), labels)
        
        return nil
    }),
)

// Add monitoring to pipeline
monitoredETL := pipz.NewSequence[ETLRecord]("monitored-etl",
    etlPipeline,
    etlMetrics,
)
```

## Testing ETL Pipelines

```go
func TestETLPipeline(t *testing.T) {
    pipeline := createETLPipeline(testConfig)
    
    testRecord := ETLRecord{
        ID:      "test-123",
        Source:  "database",
        RawData: `{"user_id": "123", "amount": 100.50}`,
    }
    
    result, err := pipeline.Process(context.Background(), testRecord)
    
    assert.NoError(t, err)
    assert.NotEmpty(t, result.Parsed)
    assert.NotEmpty(t, result.Enriched)
    assert.Empty(t, result.Errors)
    assert.Equal(t, 100.50, result.Parsed["amount"])
}

func TestETLErrorHandling(t *testing.T) {
    pipeline := resilientETL
    
    // Test with invalid data
    badRecord := ETLRecord{
        ID:      "bad-123",
        Source:  "unknown",
        RawData: "invalid json {",
    }
    
    _, err := pipeline.Process(context.Background(), badRecord)
    
    // Should handle error gracefully
    assert.Error(t, err)
    
    // Verify error was logged to DLQ
    dlqRecord, exists := dlq.Get("bad-123")
    assert.True(t, exists)
    assert.Contains(t, dlqRecord.Errors, "parse error")
}
```

## See Also

- [Sequence Reference](../reference/connectors/sequence.md)
- [Switch Reference](../reference/connectors/switch.md)
- [Concurrent Reference](../reference/connectors/concurrent.md)
- [Handle Reference](../reference/processors/handle.md)