package parser

import (
	"testing"
)

func TestCollect_1_2_EmptyInput(t *testing.T) {
	cs := Collect("", 0)
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	// Should have top-level statement keywords.
	want := []int{kwSELECT, kwINSERT, kwUPDATE, kwDELETE, kwCREATE, kwALTER, kwDROP}
	for _, tok := range want {
		if !cs.HasToken(tok) {
			t.Errorf("missing expected token %d", tok)
		}
	}
}

func TestCollect_1_2_AfterSemicolon(t *testing.T) {
	sql := "SELECT 1; "
	cs := Collect(sql, len(sql))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	// After semicolon, should have top-level statement keywords.
	want := []int{kwSELECT, kwINSERT, kwUPDATE, kwDELETE, kwCREATE, kwALTER, kwDROP}
	for _, tok := range want {
		if !cs.HasToken(tok) {
			t.Errorf("missing expected token %d for new statement after semicolon", tok)
		}
	}
}

func TestCollect_1_2_SelectCursor(t *testing.T) {
	sql := "SELECT "
	cs := Collect(sql, len(sql))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	// SELECT | should offer DISTINCT, ALL keywords and columnref, func_name rules.
	if !cs.HasToken(kwDISTINCT) {
		t.Error("missing DISTINCT keyword candidate")
	}
	if !cs.HasToken(kwALL) {
		t.Error("missing ALL keyword candidate")
	}
	if !cs.HasRule("columnref") {
		t.Error("missing columnref rule candidate")
	}
	if !cs.HasRule("func_name") {
		t.Error("missing func_name rule candidate")
	}
}

func TestCollect_1_2_CreateCursor(t *testing.T) {
	sql := "CREATE "
	cs := Collect(sql, len(sql))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	want := []int{kwTABLE, kwINDEX, kwVIEW, kwDATABASE, kwFUNCTION, kwPROCEDURE, kwTRIGGER, kwEVENT}
	for _, tok := range want {
		if !cs.HasToken(tok) {
			t.Errorf("missing expected token %d after CREATE", tok)
		}
	}
}

func TestCollect_1_2_AlterCursor(t *testing.T) {
	sql := "ALTER "
	cs := Collect(sql, len(sql))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	want := []int{kwTABLE, kwDATABASE, kwVIEW, kwFUNCTION, kwPROCEDURE, kwEVENT}
	for _, tok := range want {
		if !cs.HasToken(tok) {
			t.Errorf("missing expected token %d after ALTER", tok)
		}
	}
}

func TestCollect_1_2_DropCursor(t *testing.T) {
	sql := "DROP "
	cs := Collect(sql, len(sql))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	want := []int{kwTABLE, kwINDEX, kwVIEW, kwDATABASE, kwFUNCTION, kwPROCEDURE, kwTRIGGER, kwEVENT, kwIF}
	for _, tok := range want {
		if !cs.HasToken(tok) {
			t.Errorf("missing expected token %d after DROP", tok)
		}
	}
}
