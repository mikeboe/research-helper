package vectorstore

import "testing"

func TestIsValidTableName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid standard", "embeddings", true},
		{"Valid with underscore", "my_collection", true},
		{"Valid with numbers", "collection123", true},
		{"Valid short", "a", true},
		{"Valid max length", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_", true}, // 63 chars
		{"Invalid start with number", "1collection", false},
		{"Invalid special chars", "collection-name", false},
		{"Invalid space", "collection name", false},
		{"Invalid SQL injection", "users; DROP TABLE embeddings", false},
		{"Invalid empty", "", false},
		{"Invalid too long", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789__", false}, // 64 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidTableName(tt.input); got != tt.expected {
				t.Errorf("isValidTableName(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildMetadataQuery(t *testing.T) {
	vs := &PGVectorStore{}

	tests := []struct {
		name          string
		filter        map[string]interface{}
		wantQuery     string
		wantArgsCount int
		wantErr       bool
	}{
		{
			name:          "Empty filter",
			filter:        map[string]interface{}{},
			wantQuery:     "TRUE",
			wantArgsCount: 0,
		},
		{
			name:          "Single key-value",
			filter:        map[string]interface{}{"source": "doc1"},
			wantQuery:     "metadata @> $1",
			wantArgsCount: 1,
		},
		{
			name: "$and operator",
			filter: map[string]interface{}{
				"$and": []interface{}{
					map[string]interface{}{"a": 1},
					map[string]interface{}{"b": 2},
				},
			},
			wantQuery:     "((metadata @> $1) AND (metadata @> $2))",
			wantArgsCount: 2,
		},
		{
			name: "$or operator",
			filter: map[string]interface{}{
				"$or": []interface{}{
					map[string]interface{}{"a": 1},
					map[string]interface{}{"b": 2},
				},
			},
			wantQuery:     "((metadata @> $1) OR (metadata @> $2))",
			wantArgsCount: 2,
		},
		{
			name: "$not operator",
			filter: map[string]interface{}{
				"$not": map[string]interface{}{"a": 1},
			},
			wantQuery:     "NOT (metadata @> $1)",
			wantArgsCount: 1,
		},
		{
			name: "Nested operators",
			filter: map[string]interface{}{
				"$or": []interface{}{
					map[string]interface{}{"a": 1},
					map[string]interface{}{
						"$and": []interface{}{
							map[string]interface{}{"b": 2},
							map[string]interface{}{"c": 3},
						},
					},
				},
			},
			wantQuery:     "((metadata @> $1) OR (((metadata @> $2) AND (metadata @> $3))))",
			wantArgsCount: 3,
		},
		{
			name: "Implicit AND (multi-key map)",
			filter: map[string]interface{}{
				"a": 1,
				"b": 2,
			},
			// The query string matches because placeholders populate in visited order
			wantQuery:     "metadata @> $1 AND metadata @> $2",
			wantArgsCount: 2,
		},
		{
			name: "Error: Value for $or is not a list",
			filter: map[string]interface{}{
				"$or": "invalid",
			},
			wantErr: true,
		},
		{
			name: "Error: Item in $and list is not an object",
			filter: map[string]interface{}{
				"$and": []interface{}{
					"invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "Error: Value for $not is not an object",
			filter: map[string]interface{}{
				"$not": []interface{}{"invalid"},
			},
			wantErr: true,
		},
		{
			name: "Edge Case: Empty list in operator (ignored)",
			filter: map[string]interface{}{
				"$or": []interface{}{},
			},
			wantQuery:     "TRUE",
			wantArgsCount: 0,
		},
		{
			name: "Edge Case: Operator with empty objects",
			filter: map[string]interface{}{
				"$and": []interface{}{
					map[string]interface{}{},
				},
			},
			// recursive call returns TRUE, so we get (TRUE)
			wantQuery:     "((TRUE))",
			wantArgsCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args []interface{}
			gotQuery, err := vs.buildMetadataQuery(tt.filter, &args)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildMetadataQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotQuery != tt.wantQuery {
				t.Errorf("buildMetadataQuery() query = %q, want %q", gotQuery, tt.wantQuery)
			}
			if len(args) != tt.wantArgsCount {
				t.Errorf("buildMetadataQuery() args count = %d, want %d", len(args), tt.wantArgsCount)
			}
		})
	}
}
