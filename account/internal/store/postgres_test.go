package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFormatIdentifier(t *testing.T) {
	id := uuid.New()
	arr := [16]byte(id)
	ptrArr := new([16]byte)
	*ptrArr = arr
	pgUUID := pgtype.UUID{Bytes: arr, Valid: true}

	cases := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{name: "string", value: id.String(), want: id.String()},
		{name: "byte array", value: arr, want: id.String()},
		{name: "byte array pointer", value: ptrArr, want: id.String()},
		{name: "pgtype uuid", value: pgUUID, want: id.String()},
		{name: "pgtype uuid pointer", value: &pgUUID, want: id.String()},
		{name: "nil pointer", value: (*pgtype.UUID)(nil), wantErr: true},
		{name: "invalid pgtype", value: pgtype.UUID{}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := formatIdentifier(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil value %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
