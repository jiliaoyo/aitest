// seed 写入开发演示数据：JLPT 分类树、来源、知识点、一批已发布题目和两个账号。
// 幂等：检测到 JLPT 考试已存在时直接跳过。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "缺少 DATABASE_URL")
		os.Exit(1)
	}
	pool, err := store.Open(ctx, url)
	if err != nil {
		fail(err)
	}
	defer pool.Close()

	var examID string
	err = pool.QueryRow(ctx, `SELECT id::text FROM exams WHERE code = 'jlpt'`).Scan(&examID)
	if err == nil {
		fmt.Println("演示数据已存在，跳过 seed")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		fail(err)
	}

	err = store.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO exams (code, name) VALUES ('jlpt', '日本语能力测试') RETURNING id::text`,
		).Scan(&examID); err != nil {
			return err
		}
		levelIDs := map[string]string{}
		for i, l := range []struct{ code, name string }{
			{"n5", "N5"}, {"n4", "N4"}, {"n3", "N3"}, {"n2", "N2"}, {"n1", "N1"},
		} {
			var id string
			if err := tx.QueryRow(ctx,
				`INSERT INTO exam_levels (exam_id, code, name, sort_order) VALUES ($1, $2, $3, $4) RETURNING id::text`,
				examID, l.code, l.name, i).Scan(&id); err != nil {
				return err
			}
			levelIDs[l.code] = id
		}
		subjectIDs := map[string]string{}
		for i, s := range []struct{ code, name string }{
			{"vocabulary", "文字词汇"}, {"grammar", "语法"}, {"reading", "阅读"},
		} {
			var id string
			if err := tx.QueryRow(ctx,
				`INSERT INTO subjects (exam_id, code, name, sort_order) VALUES ($1, $2, $3, $4) RETURNING id::text`,
				examID, s.code, s.name, i).Scan(&id); err != nil {
				return err
			}
			subjectIDs[s.code] = id
		}

		// 账号
		adminHash, _ := bcrypt.GenerateFromPassword([]byte("[local seed password]"), bcrypt.DefaultCost)
		learnerHash, _ := bcrypt.GenerateFromPassword([]byte("[local seed password]"), bcrypt.DefaultCost)
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (email, email_normalized, password_hash, role, default_level_id)
			 VALUES ('admin@example.com', 'admin@example.com', $1, 'admin', $2)`,
			string(adminHash), levelIDs["n5"]); err != nil {
			return err
		}
		var learnerID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (email, email_normalized, password_hash, role, default_level_id)
			 VALUES ('learner@example.com', 'learner@example.com', $1, 'learner', $2)
			 RETURNING id::text`,
			string(learnerHash), levelIDs["n5"]).Scan(&learnerID); err != nil {
			return err
		}
		_ = learnerID

		// 来源
		var sourceID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO sources (name, kind, author, license_note, internal_note, created_by)
			 VALUES ('自建 N5 练习题集', 'self_made', '教研组', '原创内容，内部使用', '演示数据', NULL)
			 RETURNING id::text`).Scan(&sourceID); err != nil {
			return err
		}
		var sectionID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO source_sections (source_id, name, sort_order) VALUES ($1, '第 1 章 基础语法', 1) RETURNING id::text`,
			sourceID).Scan(&sectionID); err != nil {
			return err
		}

		// 知识点
		kp := map[string]string{}
		insertKP := func(subject, name, desc, mistakes, examples string) error {
			var id string
			if err := tx.QueryRow(ctx,
				`INSERT INTO knowledge_points (exam_id, level_id, subject_id, name, description, common_mistakes, examples, status)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, 'published') RETURNING id::text`,
				examID, levelIDs["n5"], subjectIDs[subject], name, desc, mistakes, examples).Scan(&id); err != nil {
				return err
			}
			kp[name] = id
			return nil
		}
		for _, k := range []struct{ subject, name, desc, mistakes, examples string }{
			{"grammar", "助词 は 与 が", "「は」表示主题，「が」表示主格与新信息。", "把新信息用「は」引出。", "私【は】学生です。／誰【が】来ましたか。"},
			{"grammar", "助词 に 与 で", "「に」表存在点与到达点，「で」表动作场所与手段。", "动作场所误用「に」。", "家【に】いる。／図書館【で】勉強する。"},
			{"grammar", "て形", "动词て形用于连接先后动作与请求。", "イ段音变规则记混。", "朝起きて、顔を洗う。"},
			{"vocabulary", "时间名词", "今天、明天、每周等高频时间词。", "每～写成「每に」。", "毎日、毎週、来週"},
			{"vocabulary", "形容词活用", "い形容词与な形容词的敬体连接。", "な形容词接名词误加「い」。", "静かな部屋／広い部屋"},
		} {
			if err := insertKP(k.subject, k.name, k.desc, k.mistakes, k.examples); err != nil {
				return err
			}
		}

		// 题目：type, stem, options, key, kpName, subject
		type q struct {
			qType   string
			stem    string
			options []map[string]string
			key     map[string]any
			kp      string
			subject string
		}
		opts := func(pairs ...[2]string) []map[string]string {
			out := []map[string]string{}
			for i, p := range pairs {
				out = append(out, map[string]string{
					"id":    string(rune('a' + i)),
					"label": string(rune('A' + i)),
					"text":  p[1],
				})
			}
			return out
		}
		questions := []q{
			{"single_choice", "この店は、駅から近い（　　）、いつも混んでいる。",
				opts([2]string{"", "にしては"}, [2]string{"", "ことから"}, [2]string{"", "に沿って"}, [2]string{"", "において"}),
				map[string]any{"optionIds": []string{"b"}}, "助词 に 与 で", "grammar"},
			{"single_choice", "わたしは毎朝七時（　　）起きます。",
				opts([2]string{"", "に"}, [2]string{"", "で"}, [2]string{"", "を"}, [2]string{"", "へ"}),
				map[string]any{"optionIds": []string{"a"}}, "助词 に 与 で", "grammar"},
			{"single_choice", "図書館（　　）べんきょうします。",
				opts([2]string{"", "に"}, [2]string{"", "で"}, [2]string{"", "が"}, [2]string{"", "と"}),
				map[string]any{"optionIds": []string{"b"}}, "助词 に 与 で", "grammar"},
			{"single_choice", "あには每朝六時（　　）おきます。",
				opts([2]string{"", "が"}, [2]string{"", "に"}, [2]string{"", "で"}, [2]string{"", "は"}),
				map[string]any{"optionIds": []string{"b"}}, "时间名词", "vocabulary"},
			{"single_choice", "このへやは（　　）です。",
				opts([2]string{"", "しずかな"}, [2]string{"", "しずかで"}, [2]string{"", "しずかの"}, [2]string{"", "しずか"}),
				map[string]any{"optionIds": []string{"a"}}, "形容词活用", "vocabulary"},
			{"single_choice", "きのう、ともだち（　　）えいがをみました。",
				opts([2]string{"", "を"}, [2]string{"", "に"}, [2]string{"", "と"}, [2]string{"", "へ"}),
				map[string]any{"optionIds": []string{"c"}}, "助词 は 与 が", "grammar"},
			{"single_choice", "「おなかがすきましたね。」　「ええ、（　　）。」",
				opts([2]string{"", "たべましょう"}, [2]string{"", "たべたくないです"}, [2]string{"", "たべています"}, [2]string{"", "たべおわりました"}),
				map[string]any{"optionIds": []string{"a"}}, "て形", "grammar"},
			{"single_choice", "每晩、おふろ（　　）はいってから、ねます。",
				opts([2]string{"", "を"}, [2]string{"", "が"}, [2]string{"", "に"}, [2]string{"", "で"}),
				map[string]any{"optionIds": []string{"a"}}, "て形", "grammar"},
			{"single_choice", "しゅくだいを（　　）ひとに聞きます。",
				opts([2]string{"", "わからないと"}, [2]string{"", "わからなければ"}, [2]string{"", "わからなくて"}, [2]string{"", "わからないで"}),
				map[string]any{"optionIds": []string{"a"}}, "て形", "grammar"},
			{"multiple_choice", "下列哪些助词可以表示动作发生的场所？（多选）",
				opts([2]string{"", "で"}, [2]string{"", "に"}, [2]string{"", "を"}, [2]string{"", "から"}),
				map[string]any{"optionIds": []string{"a", "b"}}, "助词 に 与 で", "grammar"},
			{"fill_blank", "在横线处填入合适的助词：わたしはコーヒー（＿＿）すきです。",
				nil,
				map[string]any{"acceptable": []string{"が", "は"}},
				"助词 は 与 が", "grammar"},
			{"fill_blank", "在横线处填入合适的助词：七時（＿＿）家を出ます。",
				nil,
				map[string]any{"acceptable": []string{"に"}},
				"时间名词", "vocabulary"},
			// 无标准答案题（走 AI 判定路径）
			{"short_answer", "用「たい」造一个表达自己愿望的句子。",
				nil, nil, "", "grammar"},
			// 材料题：阅读材料 + 2 个小题
			{"single_choice", "メモを見て、正しいものを選びなさい：図書館は何時に閉まりますか。",
				opts([2]string{"", "午後五時"}, [2]string{"", "午後六時"}, [2]string{"", "午後七時"}, [2]string{"", "午後八時"}),
				map[string]any{"optionIds": []string{"b"}}, "时间名词", "reading"},
			{"single_choice", "メモの内容と合っているものはどれですか。",
				opts([2]string{"", "土曜日は休みです。"}, [2]string{"", "本は二人まで借りますことができます。"}, [2]string{"", "貸出期間は二週間です。"}, [2]string{"", "コピーは無料です。"}),
				map[string]any{"optionIds": []string{"c"}}, "时间名词", "reading"},
		}

		// 材料正文
		materialContent := `【図書館からのお知らせ】\n当館の開館時間は午前九時から午後六時までです。\n土曜日と日曜日は午後五時まで、月曜日は休館日です。\n本の貸出は一人三冊まで、期間は二週間です。\nコピーは一枚十円です。`

		for _, item := range questions {
			var materialVersionID *string
			if item.subject == "reading" {
				var mID, vID string
				if err := tx.QueryRow(ctx,
					`INSERT INTO materials (created_by) VALUES (NULL) RETURNING id::text`).Scan(&mID); err != nil {
					return err
				}
				if err := tx.QueryRow(ctx,
					`INSERT INTO material_versions (material_id, version_no, title, content, created_by)
					 VALUES ($1, 1, '図書館のお知らせ', $2, NULL) RETURNING id::text`,
					mID, materialContent).Scan(&vID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx,
					`UPDATE materials SET current_version_id = $2 WHERE id = $1`, mID, vID); err != nil {
					return err
				}
				materialVersionID = &vID
			}
			var optionsAny any
			if len(item.options) > 0 {
				data, _ := json.Marshal(item.options)
				optionsAny = data
			}
			var qid, vid string
			if err := tx.QueryRow(ctx,
				`INSERT INTO questions (status, has_answer, created_by) VALUES ('draft', $1, NULL) RETURNING id::text`,
				item.key != nil).Scan(&qid); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx,
				`INSERT INTO question_versions
				   (question_id, version_no, type, stem, material_version_id, options, level_id, subject_id, source_section_id, difficulty, created_by)
				 VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, 3, NULL)
				 RETURNING id::text`,
				qid, item.qType, item.stem, materialVersionID, optionsAny,
				levelIDs["n5"], subjectIDs[item.subject], sectionID).Scan(&vid); err != nil {
				return err
			}
			if kpID, ok := kp[item.kp]; ok {
				if _, err := tx.Exec(ctx,
					`INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id) VALUES ($1, $2)`,
					vid, kpID); err != nil {
					return err
				}
			}
			if item.key != nil {
				keyData, _ := json.Marshal(item.key)
				if _, err := tx.Exec(ctx,
					`INSERT INTO answer_keys (question_version_id, value, authority, explanation, created_by)
					 VALUES ($1, $2, 'official', $3, NULL)`,
					vid, keyData, "自建解析：见知识点讲解。"); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx,
				`UPDATE questions SET current_version_id = $2, published_version_id = $2,
				        status = 'published', published_at = now() WHERE id = $1`,
				qid, vid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fail(err)
	}
	fmt.Println("seed 完成：admin@example.com / [local seed password]，learner@example.com / [local seed password]")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
