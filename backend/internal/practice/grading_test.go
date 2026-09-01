package practice

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGradeTable(t *testing.T) {
	keyChoice := &StandardKey{Value: mustJSON(t, map[string]any{"optionIds": []string{"c"}}), Authority: "official"}
	keyMulti := &StandardKey{Value: mustJSON(t, map[string]any{"optionIds": []string{"a", "c"}}), Authority: "human_verified"}
	keyFill := &StandardKey{Value: mustJSON(t, map[string]any{"acceptable": []string{"ことから"}}), Authority: "official"}

	cases := []struct {
		name      string
		qType     string
		userValue json.RawMessage
		key       *StandardKey
		want      Outcome
	}{
		{
			name: "单选正确", qType: "single_choice",
			userValue: mustJSON(t, map[string]any{"optionIds": []string{"c"}}),
			key:       keyChoice,
			want:      Outcome{Status: StatusCorrect, Source: SourceDeterministic, Authority: "official", CorrectValue: keyChoice.Value},
		},
		{
			name: "单选错误", qType: "single_choice",
			userValue: mustJSON(t, map[string]any{"optionIds": []string{"b"}}),
			key:       keyChoice,
			want:      Outcome{Status: StatusIncorrect, Source: SourceDeterministic, Authority: "official", CorrectValue: keyChoice.Value},
		},
		{
			name: "多选顺序无关", qType: "multiple_choice",
			userValue: mustJSON(t, map[string]any{"optionIds": []string{"c", "a"}}),
			key:       keyMulti,
			want:      Outcome{Status: StatusCorrect, Source: SourceDeterministic, Authority: "human_verified", CorrectValue: keyMulti.Value},
		},
		{
			name: "多选漏选", qType: "multiple_choice",
			userValue: mustJSON(t, map[string]any{"optionIds": []string{"a"}}),
			key:       keyMulti,
			want:      Outcome{Status: StatusIncorrect, Source: SourceDeterministic, Authority: "human_verified", CorrectValue: keyMulti.Value},
		},
		{
			name: "填空首尾空白与全角空格", qType: "fill_blank",
			userValue: mustJSON(t, map[string]any{"text": " ことから　"}),
			key:       keyFill,
			want:      Outcome{Status: StatusCorrect, Source: SourceDeterministic, Authority: "official", CorrectValue: keyFill.Value},
		},
		{
			name: "填空错误", qType: "fill_blank",
			userValue: mustJSON(t, map[string]any{"text": "にしては"}),
			key:       keyFill,
			want:      Outcome{Status: StatusIncorrect, Source: SourceDeterministic, Authority: "official", CorrectValue: keyFill.Value},
		},
		{
			name: "未答有权威答案", qType: "single_choice",
			userValue: nil, key: keyChoice,
			want: Outcome{Status: StatusUnanswered, Source: SourceDeterministic, Authority: "official", CorrectValue: keyChoice.Value},
		},
		{
			name: "未答无标准答案", qType: "single_choice",
			userValue: nil, key: nil,
			want: Outcome{Status: StatusUnanswered, Source: SourceDeterministic},
		},
		{
			name: "有答案但无标准答案进 AI", qType: "single_choice",
			userValue: mustJSON(t, map[string]any{"optionIds": []string{"a"}}), key: nil,
			want: Outcome{Status: StatusPending, Source: SourceAI},
		},
		{
			name: "简答题即使有参考答案也进 AI", qType: "short_answer",
			userValue: mustJSON(t, map[string]any{"text": "はい"}),
			key:       &StandardKey{Value: mustJSON(t, map[string]any{"reference": "はい"}), Authority: "official"},
			want:      Outcome{Status: StatusPending, Source: SourceAI},
		},
		{
			name: "非法选项判错误", qType: "single_choice",
			userValue: mustJSON(t, map[string]any{"optionIds": []string{"zz"}}),
			key:       keyChoice,
			want:      Outcome{Status: StatusIncorrect, Source: SourceDeterministic, Authority: "official", CorrectValue: keyChoice.Value},
		},
		{
			name: "空字符串答案不算作答", qType: "fill_blank",
			userValue: mustJSON(t, map[string]any{"text": ""}), key: keyFill,
			want: Outcome{Status: StatusUnanswered, Source: SourceDeterministic, Authority: "official", CorrectValue: keyFill.Value},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Grade(tc.qType, tc.userValue, tc.key)
			if got.Status != tc.want.Status || got.Source != tc.want.Source || got.Authority != tc.want.Authority {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			if tc.want.CorrectValue != nil && string(got.CorrectValue) != string(tc.want.CorrectValue) {
				t.Fatalf("correctValue: got %s want %s", got.CorrectValue, tc.want.CorrectValue)
			}
		})
	}
}

func TestNormalizeText(t *testing.T) {
	cases := map[string]string{
		" は  ":  "は",
		"　ＡＢ　": "AB",
		"ｶﾀｶﾅ":  "カタカナ",
		"a　 b":  "a b",
	}
	for in, want := range cases {
		if got := NormalizeText(in); got != want {
			t.Errorf("NormalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}
