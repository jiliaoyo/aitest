#!/usr/bin/env python3
"""把红蓝宝书 N1/N2/N3 扫描 PDF 转成后端 JSON 导入文件。

默认先 OCR 到 /tmp/redblue123，再从缓存构建 questions_json；不会连接数据库。
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PDF_DIR = ROOT / "pdf"
OUT_DIR = ROOT / "questions_json"
WORK_DIR = Path("/tmp/redblue123")

BOOKS = {
    "n1": {
        "level": "n1",
        "content_start": 7,
        "pdf": PDF_DIR / "红蓝宝书1000题 新日本语能力考试N1文字.词汇.文法 练习+详解  .pdf",
        "pages": 341,
        "unit_counts": [36] * 18 + [40, 42],
        "mock_starts": [247, 263, 279, 295, 311, 327],
        "mock_ranges": [(731, 775), (776, 820), (821, 865), (866, 910), (911, 955), (956, 1000)],
        "mock_end": 341,
    },
    "n2": {
        "level": "n2",
        "content_start": 7,
        "pdf": PDF_DIR / "红蓝宝书1000题  新日本语能力考试N2文字.词汇.文法  练习+详解.pdf",
        "pages": 337,
        "unit_counts": [36] * 18 + [40, 42],
        "mock_starts": [247, 265, 283, 301, 319],
        "mock_ranges": [(731, 784), (785, 838), (839, 892), (893, 946), (947, 1000)],
        "mock_end": 336,
    },
    "n3": {
        "level": "n3",
        "content_start": 8,
        "pdf": PDF_DIR / "红蓝宝书1000题·新日本语能力考试N3文字·词汇·文法.pdf",
        "pages": 337,
        "unit_counts": [36] * 18 + [31, 31],
        "mock_starts": [248, 264, 280, 302, 319],
        "mock_ranges": [(711, 768), (769, 826), (827, 884), (885, 942), (943, 1000)],
        "mock_end": 336,
    },
}

ITEM_KEYS = {
    "rawExcerpt", "materialKey", "type", "stem", "options", "materialTitle",
    "materialContent", "levelCode", "subjectCode", "difficulty",
    "knowledgePointNames", "sourceAnswer", "aiSuggestedAnswer", "anomalies",
}
QUESTION_PROMPT = "<image>\nTranscribe every Japanese question number and all four answer choices exactly. Do not summarize."
ANSWER_PROMPT = "<image>\nRead only the answer-key row at the top of the page. Return the question range and its answer digits, for example: 001-006: 2 3 1 4 2 1. Ignore all explanations."
QUESTION_RE = re.compile(r"(?m)^\s*(\d{3,4})(?=\s|[.．、)])")
OPTION_RE = re.compile(r"(?m)(?:^|\s)([1-4])(?:[.．、:：)]|[ \t]+|(?=[\u3040-\u30ff\u3400-\u9fffA-Za-z]))")
SECTION_RE = re.compile(r"(?:問題|问题)\s*([1-9])")
ANSWER_RE = re.compile(
    r"(?<!\d)(\d{3,4})\s*(?:\[?【?\s*答\s*案\s*】?\]?|答\s*案)\s*[:：]?\s*([1-4])(?!\d)"
)
NOISE_RE = re.compile(r"(?:aws|attsu?|角和|image|text|hydraulic|press|\ufffd|[<>])", re.I)
OCR_PROMPT_MARKER_RE = re.compile(
    r"(?:the text or|or paraphrase the text|or use external resources|"
    r"or use images of any kind|or make any assumptions about the content of the image)",
    re.I,
)
OCR_TAIL_RE = re.compile(
    r"\s+(?:[-—]{1,}\s*)?(?:Unit\b|Question\b|Image Description\b|"
    r"the text or|or paraphrase the text|or use external resources|"
    r"or use images of any kind|or make any assumptions about the content of the image|"
    r"(?:問題|问题)(?:\s*\d{0,4})?|(?:文字|語彙|文法)(?:\s*\d{0,4})?|"
    r"正解|解答|答案).*?$|\s+\d{3}\s+.*$|\s+\d{3}(?:\s+\d{3})?\s*$",
    re.I,
)
MANUAL_OPTION_FIXES_N3 = {
    764: ["という", "に対する", "としての", "による"],
    765: ["あっても", "あるのに", "あったきり", "あったつもりで"],
    766: ["ところで", "たとえば", "一方", "それに"],
    767: ["にかわって", "からみて", "とともに", "みたいに"],
    822: ["あそこで", "あそこに", "そこで", "そこに"],
    823: ["ことにしました", "ことになりました", "ものにしました", "ものになりました"],
    824: ["起きている", "起きていた", "避けられる", "避けられた"],
    825: ["聞くたびに", "聞いたばかりで", "聞くだけでは", "聞いていると"],
    875: ["ピアノを", "習っていた", "ながらも", "いやいや"],
    876: ["恐れ", "11日に", "大雨となる", "かけて"],
    877: ["大人たちが", "電車などで", "漫画を読んでいる姿", "夢中になって"],
    880: ["つかわされた", "つかった", "つかれれた", "つかわせた"],
    904: ["インスタント", "インタビュー", "インターネット", "インフルエンザ"],
    938: ["あってしょうがないでしょうか", "あったらどうでしょうか", "あるはずがないでしょうか", "あるのではないでしょうか"],
    939: ["だけでなく", "である以上", "をはじめとして", "に対して"],
    940: ["すえに", "ために", "おかげで", "一方で"],
    941: ["見たりするべきではありません", "見たりしてもはじまりません", "見たりしてはいけません", "見たりしてもかまいません"],
    975: ["4月に入って、天気は次々暖かくなってきた。", "金魚が水槽の中で次々に泳いでいる。", "こんな簡単なことは次々説明する必要はありません。", "参加者たちが次々と会場に入ってきました。"],
    996: ["にかかって", "には及ばない", "に限ったことではない", "に過ぎない"],
    997: ["工夫させたい", "工夫してやりたい", "工夫させてもらいたい", "工夫してもらいたい"],
    998: ["これに対し", "あれに対し", "それとも", "それでも"],
    999: ["だけ", "とか", "さえ", "まで"],
}
MANUAL_ITEMS_N2 = {
    7: ("薬局で風邪薬をもらった。", ["くすりや", "やくや", "くすりきょく", "やっきょく"]),
    8: ("頂上に近づくにつれて、山道が険しくなる。", ["くわしく", "けわしく", "ただしく", "おかしく"]),
    288: ("「課長、企画書についてご意見を（    ）たいんですが、お時間よろしいですか。」", ["お訪ねし", "ご覧に入れ", "うかがい", "おいでになり"]),
    784: ("このデータを集めていくと、見事にお客様一人ひとりの好みの特徴がわかってくる。最後に、そのお客様が嫌いなものは出さないで、好きなものだけ出すようにする。料理を食べ残すことから、繰り返しを使うことでお客様の好き嫌いを（    ）。", ["発見しなければならない", "発見するといい", "発見することがある", "発見したのである"]),
    886: ("沖縄 ______ ★ ______ 有名ですよね。", ["ことで", "といえば", "きれいな", "海が"]),
    937: ("世の中には、______ ★ ______ ことがたくさんある。", ["経験すれば", "実際に", "こそ", "本当に理解できる"]),
    1000: ("そこで毎日、新しい仕事からスタートするようにすれば、（    ）飛躍をするかもしれません。", ["思いがけない", "まぶしい", "頼もしい", "いさましい"]),
    76: ("４月１日からガス代や電気代などの公共料金が（    ）に値上がりする。", ["一向", "一斉", "一時的", "一方的"]),
    147: ("３歳の息子はおもちゃの車を（    ）するのが大好きだ。", ["分配", "分別", "分離", "分解"]),
    433: ("３時間前の大雨警報が解除になった。", ["かいしょ", "かいじょ", "がいじょ", "がいしょ"]),
    656: ("２歳の息子はよく絵本を逆様にして見ている。", ["さかさま", "さかざま", "ぎゃくざま", "ぎゃくよう"]),
    836: ("紙に書いてポストに入れる手紙は（    ）２０世紀の科学技術の中には恩恵とばかりは言いがたいものがたくさんあるが、電子メールは確かにすばらしい。", ["書かなくもない", "書かなくなるだろう", "書くどころではない", "書いてもかまわないだろう"]),
    838: ("何らかの形で、互いの実りのある情報交換ができる（    ）自分自身が何を欲しているかを自分で知っていなければならない。", ["かどうか", "からこそ", "ためには", "と同時に"]),
    643: ("中村さんはようやく事件の始終を語った。", ["じじゅ", "じじゅう", "しじゅ", "しじゅう"]),
    644: ("彼はチャンバンジーのまねをして、滑稽な顔をした。", ["こつけい", "こっけい", "かつけい", "かっけい"]),
    645: ("佐藤くんはよく頭痛を（    ）に、授業をサボる。", ["言い訳", "言い合い", "言い伝え", "言い分"]),
    646: ("期限までに授業料を（    ）と、入学できない。", ["すませない", "かぞえない", "おさめない", "あずけない"]),
    647: ("子どもたちの気持ちも考えて（    ）。", ["あげてください", "さしあげてください", "もらってください", "いただいてください"]),
    648: ("「この問題について、会長のお考えを（    ）。」", ["お聞き差し上げませんか", "お聞きしてくださいませんか", "お聞かせ申し上げますか", "お聞かせ願えないでしょうか"]),
    662: ("週末は、子どもを連れて郊外へ遠足に出かける予定だ。", ["とおあし", "どおあし", "えんそく", "えんぞく"]),
    663: ("新人研修は今年も従来どおり、東京の本社で行います。", ["しゅらい", "しゅうらい", "じゅらい", "じゅうらい"]),
    664: ("友だちの新築祝いにおしゃれな食器をプレゼントした。", ["しょくき", "しょっき", "しょくぎ", "しょうき"]),
    665: ("先輩の話を（    ）に、留学準備を進める。", ["解釈", "解説", "参考", "参照"]),
    666: ("夫は電気屋で部品を買ってきて、自分でパソコンを（    ）。", ["組み立てた", "組み込んだ", "組み入れた", "組み合わせた"]),
    667: ("山田さんが辞めた（    ）、ほんとう？", ["っけ", "って", "った", "か"]),
    668: ("言葉はコミュニケーションの道具の一つで（    ）。", ["しかあるまい", "はあるまい", "ほかあるまい", "だけあるまい"]),
    887: ("A「ねえ、何かおいしいものをご馳走して（    ）ないんだけどね。」B「いいよ、買い物が終わったら、焼肉食べに行こう。」", ["くれるなら", "でも", "買い物に", "付き合わない"]),
    888: ("日本語の青信号という言葉は（    ）。", ["間違っていますね", "正しいですね", "間違いなのでしょうか", "正しいのでしょうか"]),
    889: ("呼び方（    ）、色に対する日本人の美意識が表現されているといっても過言ではないのです。", ["にこそ", "にさえ", "によって", "にだけ"]),
    890: ("（    ）、外国では信号の3色をどう呼んでいるのか、調べてみました。", ["要するに", "そして", "とはいえ", "ところで"]),
    891: ("でも、（    ）「青信号」については、ほとんどの国の人が「緑」だと答えたのです。", ["あくまで", "少なくとも", "特に", "実に"]),
    892: ("唯一、ネパール人だけが「緑ではなく、空の色を示す青を使っている」と答えてくれました。（    ）、「ネパールの信号の技術は、日本から導入された」とのこと。", ["周知のように", "いずれにしても", "よく聞いてみると", "話を聞いているうちに"]),
    938: ("これほどのベストセラーになるとは、______ ★ ______", ["彼女自身", "予想し得なかった", "この本を書いた", "にしたって"]),
    939: ("明日から決して ______ ★ ______ 決意をした。", ["彼は", "吸うまいと", "タバコを", "禁煙の"]),
    940: ("最近、父はゴルフを ______ ★ ______ ようだ。", ["たまらない", "ばかりで", "始めた", "面白くて"]),
    941: ("彼女と ______ ★ ______ ことがある。", ["告白する決意をした", "と思ったとき", "かもしれない", "もう二度と会えない"]),
    942: ("しかし、それは同じくらいの年齢や家族構成でないと友だちになれないというのは（    ）。", ["少し違います", "全く違います", "同じです", "関係があるようです"]),
    943: ("いつか、（    ）年を重ねたいという素敵な「人生の先輩」を友だちにしてしまう。", ["こんなふうに", "それぐらい", "あんなふうに", "このように"]),
    944: ("先輩でなく友だちでお願いします」というのは（    ）。", ["とてもいいこと", "ちょっと無理", "よくあること", "確かに簡単"]),
    945: ("相手はかなり年上だけれど講座においては（    ）、というポジションになります。", ["同級生", "人生の先輩", "仲間", "魅力的な人"]),
    946: ("きっと「ナイスアドバイスをありがとう」「（    ）。", ["と思えないのでしょう", "と思えるのではないでしょうか", "と思われるのでしょうか", "と思われないでしょう"]),
    780: ("そして（    ）が次に来るときには、前回食べ残したような料理は出さない。", ["新しいお客様", "そのお客様", "昔のお客様", "このお客様"]),
    781: ("（    ）、毎回同じ料理を出すわけではないので、季節によってさまざまな料理を出すわけだ。", ["というのは", "すなわち", "とりあえず", "もちろん"]),
    782: ("実際にはそれは（    ）、自分がおいしいと思うものだけが出されていたのである。", ["おいしいかもしれないが", "おいしいのではなく", "おいしいとは言えないが", "おいしいのは言うまでもなく"]),
    783: ("（    ）、そのお客様が嫌いなものは出さないで、好きなものだけ出すようにする。", ["それに加え", "それなら", "その上で", "その前に"]),
    833: ("武田「小野さん、______ ★ ______。」小野「どうもすみません。」", ["教えてくれなくちゃ", "社員旅行に", "行けないのなら", "もっと早く"]),
    834: ("どんな天才でも、とくに現代の科学では、たった一人で考えている（    ）、あまり実りのあるものは出てこない。", ["からには", "かわりに", "だけでは", "といっても"]),
    837: ("（    ）通信・情報の手段がどのように変わっても、有意義なコミュニケーションの基本は変わらないだろう。", ["つまり", "しかし", "したがって", "しかも"]),
    994: ("日本人にとって ______ ★ ______ 生活習慣の一つです。", ["体を清潔に保つほか", "疲れをとったり", "心をリラックスさせるための", "お風呂とは"]),
    996: ("私はマスコミで長い間（    ）が、もともと深夜型である上に、睡眠の絶対量が人より少ないようで、週刊誌の仕事にもっとも合っていました。", ["働いていきたいです", "働いていません", "働いてきました", "働いています"]),
    997: ("睡眠の絶対量が人より（    ）ようで、週刊誌の仕事にもっとも合っていました。", ["多くてたまらない", "少なくてたまらない", "多くてすむ", "少なくてすむ"]),
    998: ("これは週刊誌の一例（    ）、ときに一定のリズムを自分に課していくと、非常に大きな収穫を得ることになります。", ["にすぎないので", "にすぎませんが", "だけに", "のみならず"]),
    999: ("新しい試みや仕事は後回しにされた（    ）、結局やれないことになってしまいます。", ["とたん", "つもりで", "ところが", "まま"]),
}
MANUAL_ITEMS_N1 = {
    204: ("旅行に行ける（    ）行きたいけど、どうも休みが取れない。", ["ともなしに", "ものなら", "限り", "ように"]),
    360: ("先輩、会議時間の変更を山田課長に（    ）。", ["お伝え申し上げませんか", "お伝えしませんか", "伝えていらっしゃいますか", "お伝え願えませんか"]),
    540: ("「課長、こちらはいつもお世話になっているA社の野原部長（    ）。」", ["ございます", "でございます", "でいらっしゃいます", "おいでになります"]),
    955: ("本当は「わからない」くせに、勢いで「わかる」と言ってしまうと、あとで恥をかきます。そういうときは、正直に「わかりません」と認めるのが、実直な態度ではないでしょうか。（    ）", ["もっとも", "ところで", "さて", "したがって"]),
    207: ("３年間の貯金を留学費用に（    ）。", ["預ける", "充てる", "貯める", "注ぐ"]),
    370: ("３歳の息子が靴を（    ）に履いている。", ["あれこれ", "あべこべ", "ちやほや", "でこぼこ"]),
    457: ("３日も徹夜の作業が続き、もう倒れる寸前だ。", ["すんまえ", "すんぜん", "ずんまえ", "ずんせん"]),
    593: ("２か月の旅行から帰ってきたら、家中がほこり（    ）になっていた。", ["ぐるみ", "ずくめ", "まみれ", "の極み"]),
    637: ("３歳の娘は自我が芽生え、自己主張が強くなった。", ["しか", "しが", "じか", "じが"]),
    661: ("２人の候補者は火花を散らす舌戦を繰り広げた。", ["かか", "かはな", "ひばな", "ひはな"]),
    772: ("何年かある会社で働いていると、仕事ができる、できないという評価ははっきり（    ）。", ["ところです", "べきです", "ことです", "ものです"]),
    774: ("読書さえしていれば、そのような悲劇は（    ）はずなのです。", ["避けられた", "避けられなかった", "避けられる", "避けられない"]),
    818: ("幸運というのは、言うなれば（    ）ただの入場券のようなものです。", ["しかし", "だから", "しかも", "要するに"]),
    877: ("２か月にわたる交渉が（    ）に終わった。", ["不順", "不快", "不況", "不調"]),
    957: ("１週間練習をしてやっと自転車の乗り方のコツを会得した。", ["かくとく", "しゅうとく", "かいとく", "えとく"]),
    771: ("あなたの周りには、「この人は本当に仕事ができないな」と感じてしまう人はいませんか？（    ）人を前にすると、忠告したくなってしまいます。", ["あのような", "そのような", "あの", "その"]),
    773: ("それでも、「給料さえもらえていればいい」と割り切っているのならば、まだいいでしょう。（    ）悲しいことに、そういう人のほとんどは自分の状況が理解できていません。", ["というのは", "また", "ところが", "むしろ"]),
    817: ("「強い豊かな才能があれば、それは必ずいつか花開くものだ」と主張する人もいます。しかし僕の実感からいえば、必ずしも（    ）。", ["そうとばかり思っていたのです", "そうでなければならないみたいです", "そう思うことはない", "そうとは限らないようです"]),
    819: ("その入場券を持っていれば、あなたは催し物の会場に（    ）——でもそれだけのことです。", ["入れてもらえます", "入ってくれます", "入れてくれます", "入ってもらえます"]),
    851: ("不景気の波は中小企業（    ）、大手企業にまで及んだ。", ["にかかわらず", "にすぎなくて", "にとどまらず", "に及ばず"]),
    861: ("「友情・恋愛マンガ」を「よく読む」と答えたのは、女子が小56％・中70％・高62％だった（    ）、男子は12％・20％・36％。", ["かわりに", "のに伴い", "のと同じく", "のに対し"]),
    862: ("教育的要素を持つマンガはそれほど好まれていないが、学年が上がるにつれて「よく読む」の割合が（    ）。", ["少しずつ増えてくる", "急に増えてくる", "さらに減っていく", "ほとんど変わらない"]),
    863: ("「今まで知らなかったことがわかった」も小中高の7割以上が「はい」と答えており、（    ）、勉強とは違う知識習得の効用も認識されているようだ。", ["一方", "つまり", "たとえば", "次に"]),
    864: ("マンガに否定的な項目では小中高ともに9割前後が「（    ）」と答えた。", ["はい", "いいえ", "よく読む", "あまり読まない"]),
    884: ("田中さんはおおざっぱな性格だ。", ["すぐ怒る", "内向的な", "細かいことを気にしない", "自分の考えをはっきり言う"]),
    888: ("進呈", ["高速鉄道の建設工事は大きく進呈した。", "１万円以上お買い上げのお客様に500円のクーポンを進呈します。", "奨学金の申込書は月末までに学生課に進呈してください。", "そろそろ卒業後の進呈について考えなければならない。"]),
    906: ("だから、新年（    ）何か目標を立てたり、新しいことをはじめるということもない。", ["にいたって", "にあたって", "にしたって", "にかぎって"]),
    907: ("新年を新しい事始めにすることが間違っているとか、意味がないと（    ）。", ["思っているわけではない", "思えてならない", "思っているのである", "思えてしまいそうだ"]),
    908: ("年が明けて、（    ）問題が新しくなるわけではないからだ。", ["いわゆる", "あらゆる", "いわば", "あるいは"]),
    909: ("若い者には負けないと体力の限界に挑み、急斜面の雪面を滑り降りるとか、冬の海で泳いだり潜ったりするとか、「（    ）」。", ["そんなことに大きな意味がある", "そんなことは絶対にしない", "そんなことは考えなくもない", "そんなことに挑戦してみたい"]),
    951: ("このことばを言われるたび、子ども（    ）そういうものなのかなと思ったのを思い出します。", ["なりに", "らしく", "ながらに", "として"]),
    952: ("母の勧める本は子どもの私には難解なうえ、旧字体で書かれているので、辞書を（    ）読めません。", ["引きながらでいては", "引きながらでないと", "引いてみたら", "引かずとも"]),
    953: ("何度も読み返してみるものの、やはり子どもですから限界があります。（    ）母に「やっぱりわからないよ」と言うと、決まって母は冒頭のことばを口にするのです。", ["そこで", "だから", "しかも", "にもかかわらず"]),
    954: ("ある人が自分で「わかった」と思っていても、別の人からみると「（    ）」となることは、ざらにあります。", ["全然わかっていない", "全然わからない", "全然わかっていなかった", "全然わからなかった"]),
    996: ("ただ、ピークの85年の体力（    ）。", ["にかかっている", "には及ばない", "に限ったことではない", "に過ぎない"]),
    997: ("学校には調査結果を基に、指導をさらに（    ）。", ["工夫させたい", "工夫してやりたい", "工夫させてもらいたい", "工夫してもらいたい"]),
    998: ("運動をしている高齢者の8割以上は、何にもつかまらずにズボンやスカートがはけた。（    ）、運動をしない高齢者では7割弱だった。", ["これに対し", "あれに対し", "それとも", "それでも"]),
    999: ("ただ、地元のクラブの存在（    ）知らない住民が多い。", ["だけ", "とか", "さえ", "まで"]),
    1000: ("体を動かし、汗を流す人が（    ）、知恵を絞りたい。", ["増えるよう", "増えよう", "増やすよう", "増やそう"]),
}
MANUAL_ITEMS_N3 = {
    768: ("時間をどのように感じ、どのように行動するかも、文化によって異なり、ある文化で「よい」ことが他の文化では「よくない」ことになることも（    ）。", ["あるわけではありません", "あってはなりません", "少なくありません", "少ないはずです"]),
    881: ("日本人は、（    ）旅行に行ったときは、必ずと言っていいほど職場の人や友だちにおみやげを買ってきます。", ["なにかに", "どこかに", "だれかに", "いつかに"]),
    942: ("（    ）、いざ始めてしまえば、意外とすんなり進めることができたりします。", ["ですから", "しかも", "また", "しかし"]),
    1000: ("日本全国で行われる日本人との交流に関する情報も、（    ）そうです。", ["紹介しているところだ", "紹介していくつもりだ", "紹介したところだ", "紹介したつもりだ"]),
    54: ("３日ぶりに愛犬が傷（    ）で帰ってきました。", ["かかり", "ながら", "くらい", "だらけ"]),
    75: ("３日でこの仕事を終わらせるのは（    ）です。", ["無駄", "無理", "懸命", "大変"]),
    294: ("３歳の息子が歯磨きを（    ）、毎日大変だ。", ["嫌がらで", "嫌がって", "嫌ぎみで", "嫌がると"]),
    496: ("１年間絶交していたあの二人は（    ）仲直りしました。", ["ついに", "たまに", "つねに", "とくに"]),
    882: ("そして、おみやげをもらったほうは、「すみませんお気づかいいただいて……」と言いながら（    ）、自分が旅行に行ったときは……。", ["買ってすみます", "買ってみせます", "買ってきます", "買っていきます"]),
    883: ("もらいながらも、（    ）自分が旅行に行ったときは、「この前おみやげをもらったから……」と言って、また相手に自分のおみやげを渡す。", ["今ごろ", "次に", "先ごろ", "前に"]),
    884: ("（    ）、日本人の「おみやげ文化」は、おみやげを渡すほうも、もらうほうも、相手に対して気をつかうことで、成り立っているのです。", ["つまり", "とはいっても", "しかし", "なにしろ"]),
}
MANUAL_ANSWERS_N1 = {
    204: 2, 207: 2, 360: 2, 370: 2, 457: 2, 540: 3, 593: 3, 637: 4, 661: 3, 772: 4, 774: 1, 818: 1, 877: 4, 955: 4, 957: 3,
    771: 2, 773: 3, 817: 4, 819: 1, 851: 3,
    861: 4, 862: 3, 863: 1, 864: 2, 884: 3, 888: 2,
    906: 2, 907: 1, 908: 2, 909: 2, 951: 3, 952: 2, 953: 1, 954: 1,
    996: 2, 997: 4, 998: 1, 999: 3, 1000: 1,
}
MANUAL_ANSWERS_N2 = {
    7: 4, 8: 2, 288: 3,
    76: 2, 147: 4, 433: 2, 656: 1, 784: 4, 836: 2, 838: 3, 886: 3, 937: 3,
    662: 3, 663: 4, 664: 2, 665: 3, 666: 1, 667: 2, 668: 1,
    887: 4, 888: 3, 889: 1, 890: 4, 891: 2, 892: 3,
    780: 2, 781: 4, 782: 2, 783: 3, 833: 4, 834: 3, 837: 2,
    938: 4, 939: 1, 940: 4, 941: 2, 942: 1, 943: 3, 944: 2, 945: 1, 946: 2,
    994: 2, 996: 3, 997: 4, 998: 2, 999: 4, 1000: 1,
}
MANUAL_ANSWERS_N3 = {54: 4, 75: 2, 294: 2, 496: 1, 768: 3, 881: 2, 882: 3, 883: 2, 884: 1, 942: 4, 1000: 2}

MANUAL_ANSWERS_N1.update({
    35: 3, 36: 2, 41: 2, 42: 2, 72: 4, 102: 1,
    118: 4, 119: 4, 120: 3,
    169: 3, 170: 2, 171: 1, 172: 4, 173: 3, 174: 1,
    216: 4, 276: 3,
    301: 4, 302: 1, 303: 3, 304: 2, 305: 3, 306: 1,
    312: 2, 323: 3, 324: 2,
    331: 2, 332: 1, 333: 3, 334: 4, 335: 1, 336: 4,
    353: 4, 354: 3, 359: 4,
    388: 1, 389: 2, 390: 4, 396: 4, 462: 3,
    511: 4, 512: 2, 513: 3, 514: 1, 515: 2, 516: 3,
    527: 2, 528: 3, 588: 2,
    596: 4, 597: 3, 598: 2, 599: 2, 600: 3,
    607: 3, 608: 2, 609: 1, 610: 4, 611: 3, 612: 4,
    638: 1, 639: 3, 640: 2, 641: 1, 642: 4,
    675: 3, 676: 1, 677: 4, 678: 2, 679: 1, 680: 2, 681: 4,
    687: 1, 688: 4,
    707: 3, 708: 4, 709: 3, 723: 2,
    755: 2, 760: 3,
    803: 2, 804: 4, 814: 3,
    828: 2, 829: 4, 830: 1, 831: 3, 832: 4, 833: 3, 834: 2,
    850: 4, 853: 1, 854: 2,
    870: 2, 871: 1, 872: 2,
    889: 1, 890: 3, 900: 2, 901: 3, 905: 4, 910: 3, 918: 3, 919: 1, 920: 4,
    925: 1, 926: 3,
    935: 2, 937: 3, 944: 2,
    967: 2, 970: 1, 971: 2,
    985: 2, 986: 1, 987: 3, 990: 2, 995: 1,
})

MANUAL_ANSWERS_N2.update({
    4: 2, 5: 4, 6: 1, 23: 4, 24: 3,
    31: 2, 32: 3, 33: 3, 34: 2, 35: 1, 36: 4,
    67: 3, 68: 2, 69: 4, 70: 4, 71: 1, 72: 3,
    101: 4, 102: 2,
    121: 3, 122: 1, 123: 4, 124: 2, 125: 3, 126: 4,
    156: 1, 173: 2, 174: 1, 204: 1, 294: 4,
    301: 2, 302: 1, 303: 3, 304: 4, 305: 1, 306: 2,
    341: 1, 342: 3,
    413: 4, 414: 2, 419: 1, 420: 4, 425: 1, 426: 3, 444: 1,
    475: 4, 476: 1, 477: 2, 478: 3, 479: 1, 480: 3,
    485: 4, 486: 3, 582: 1,
    589: 2, 590: 3, 591: 4, 592: 3, 593: 3, 594: 2,
    595: 4, 596: 1, 597: 2, 598: 1, 599: 4, 600: 1,
    604: 4, 605: 2, 606: 1,
    613: 2, 614: 1, 615: 3, 616: 4, 617: 3, 618: 1,
    703: 2, 704: 3, 705: 4, 706: 1, 707: 4, 708: 1, 709: 2,
    731: 3, 732: 4, 733: 1, 734: 2, 735: 1, 736: 3, 737: 4, 738: 1,
    763: 1, 764: 3,
    785: 4, 786: 2, 787: 3, 788: 1, 789: 3, 790: 3, 791: 2, 792: 4,
    799: 3, 817: 1, 818: 4,
    822: 1, 823: 1, 824: 2, 835: 1,
    839: 3, 840: 4, 841: 1, 842: 4, 843: 2, 844: 2, 845: 1,
    862: 2, 863: 4, 864: 1,
    870: 4, 871: 4, 872: 2,
    893: 1, 894: 4, 895: 2, 896: 3, 897: 1, 898: 2, 899: 1,
    913: 3, 914: 2, 915: 1,
    927: 4, 928: 2, 929: 1, 930: 4,
    947: 1, 948: 3, 949: 4, 950: 2, 951: 4, 952: 3,
    955: 3, 962: 2, 967: 3,
    973: 4, 974: 3, 978: 2, 979: 2,
    988: 3, 989: 1, 990: 2,
})

MANUAL_ANSWERS_N3.update({
    7: 4, 8: 2, 9: 2, 10: 1, 11: 3, 12: 1,
    25: 2, 26: 3, 27: 2, 28: 4, 29: 1, 30: 2,
    31: 3, 32: 1, 33: 1, 34: 4, 35: 3, 36: 2,
    77: 4, 78: 3, 101: 1, 102: 3, 120: 1,
    148: 1, 149: 3, 150: 1,
    157: 2, 158: 4, 159: 3, 160: 1, 161: 4, 162: 3,
    227: 1, 228: 3,
    235: 1, 236: 3, 237: 2, 238: 4, 239: 2, 240: 3,
    276: 4, 293: 4,
    301: 2, 302: 3, 303: 2, 304: 4, 305: 1, 306: 2,
    325: 3, 326: 2, 327: 1, 328: 3, 329: 2, 330: 3,
    343: 4, 344: 2, 345: 1, 346: 3, 347: 1, 348: 4,
    367: 1, 368: 2, 369: 2, 370: 3, 371: 4, 372: 2,
    503: 4, 504: 3, 516: 2,
    529: 3, 530: 1, 531: 2, 532: 3, 533: 2, 534: 3,
    541: 3, 542: 2, 543: 1, 544: 4, 545: 1, 546: 2,
    564: 4,
    590: 4, 591: 1, 592: 2, 593: 3, 594: 1,
    670: 4, 671: 1, 672: 2, 673: 4, 674: 3,
    695: 4, 705: 2,
    706: 3, 707: 2, 708: 1, 709: 3, 710: 4,
    728: 2, 729: 1, 730: 2, 731: 1, 732: 4, 733: 3,
    734: 3, 735: 2, 736: 1, 737: 4, 738: 1, 739: 4,
    740: 3, 741: 4, 742: 1, 743: 4, 744: 2, 745: 4,
    746: 2, 747: 3, 748: 1, 749: 1, 750: 3,
    754: 2, 755: 4, 756: 3,
    761: 1, 762: 3, 763: 1, 766: 3,
    776: 2, 777: 4, 778: 2, 779: 3, 780: 1, 781: 2, 782: 4, 783: 3, 784: 1,
    794: 4,
    804: 3, 805: 4, 806: 2, 807: 4, 808: 4,
    809: 2, 810: 3, 811: 1, 812: 2, 813: 3,
    814: 2, 815: 1, 816: 3, 817: 3, 818: 4,
    824: 2, 825: 4, 826: 3,
    831: 3, 832: 1, 833: 2, 834: 3, 835: 4, 838: 1,
    843: 1, 844: 4, 845: 3, 846: 2, 847: 1, 848: 3,
    849: 3, 850: 4, 851: 2, 852: 1, 853: 4, 854: 1,
    861: 1, 862: 1, 863: 2, 864: 4, 865: 2,
    866: 2, 867: 3, 868: 1, 869: 4, 870: 2,
    871: 3, 872: 1, 873: 1, 874: 3,
    885: 3, 886: 1, 887: 2, 899: 1,
    908: 4, 909: 3, 910: 3, 911: 1, 912: 1, 913: 4,
    914: 3, 915: 4, 916: 2, 917: 3, 918: 3, 919: 1,
    920: 3, 921: 4, 922: 2, 923: 2, 924: 1, 925: 2, 926: 1,
    927: 3, 928: 2, 934: 3, 939: 1,
    943: 3, 944: 2, 945: 1,
    950: 3, 951: 2, 952: 1, 953: 4, 954: 3, 955: 2, 956: 2,
    957: 3, 958: 2, 959: 3, 960: 2, 961: 1, 962: 4, 963: 3,
    964: 2, 965: 1,
    977: 1, 978: 2, 979: 2,
    983: 3, 984: 1, 985: 1, 986: 2, 987: 2, 988: 2, 989: 1,
    990: 1, 991: 4, 992: 3,
    998: 2, 999: 1,
})


def normalize_text(value: str) -> str:
    value = unicodedata.normalize("NFKC", value)
    value = value.replace("\r", " ").replace("\u00a0", " ")
    value = value.replace("**", "")
    value = re.sub(r"```[^\n]*|```", "", value)
    value = re.sub(r"<br\s*/?>", " ", value, flags=re.I)
    lines = []
    for line in value.splitlines():
        line = re.sub(r"^\s*#{1,6}\s*", "", line)
        line = line.strip(" |`*")
        if line:
            lines.append(line)
    return "\n".join(lines)


def compact(value: str) -> str:
    value = normalize_text(value)
    value = re.sub(r"\s+", " ", value).strip(" |-*#")
    return value


def sanitize_ocr_fragment(value: str) -> tuple[str, bool]:
    compacted = compact(value)
    cleaned = OCR_TAIL_RE.sub("", compacted).strip()
    return cleaned, cleaned != compacted


def page_number(path: Path) -> int:
    match = re.search(r"page(\d+)", path.name)
    if not match:
        raise ValueError(path)
    return int(match.group(1))


def book_dir(book: str) -> Path:
    path = WORK_DIR / "ocr" / book
    path.mkdir(parents=True, exist_ok=True)
    return path


def render_page(doc, number: int, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.exists() and target.stat().st_size:
        return
    doc[number - 1].get_pixmap(dpi=200, alpha=False).save(target)


def load_ocr_model():
    os.environ["HF_HUB_OFFLINE"] = "1"
    sys.path.insert(0, str(ROOT / "scripts"))
    from ocr_utils import load_ocr

    return load_ocr()


def ocr(model, processor, generate, image: Path, *, tokens: int, penalty: float) -> str:
    result = generate(
        model,
        processor,
        prompt="<image>\nFree OCR.",
        image=str(image),
        max_tokens=tokens,
        verbose=False,
        repetition_penalty=penalty,
    )
    return result.text if hasattr(result, "text") else str(result)


def ocr_question(model, processor, generate, image: Path) -> str:
    result = generate(
        model,
        processor,
        prompt=QUESTION_PROMPT,
        image=str(image),
        max_tokens=2600,
        verbose=False,
        repetition_penalty=1.2,
    )
    return result.text if hasattr(result, "text") else str(result)


def ocr_answer_key(model, processor, generate, image: Path) -> str:
    result = generate(
        model,
        processor,
        prompt=ANSWER_PROMPT,
        image=str(image),
        max_tokens=240,
        verbose=False,
        repetition_penalty=1.2,
    )
    return result.text if hasattr(result, "text") else str(result)


def write_if_missing(path: Path, value: str) -> None:
    if not path.exists() or not path.read_text(encoding="utf-8").strip():
        path.write_text(value.strip(), encoding="utf-8")


def crop_top(image: Path, target: Path) -> None:
    from PIL import Image, ImageEnhance

    with Image.open(image) as source:
        width, height = source.size
        crop = source.crop((0, 0, width, int(height * 0.18))).convert("L")
        crop = ImageEnhance.Contrast(crop).enhance(1.4)
        crop.save(target)


def crop_halves(image: Path, directory: Path, number: int) -> tuple[Path, Path]:
    from PIL import Image

    with Image.open(image) as source:
        width, height = source.size
        paths = []
        for index, (top, bottom) in enumerate(((0, int(height * 0.56)), (int(height * 0.44), height)), 1):
            target = directory / f"page{number:03d}.part{index}.png"
            if not target.exists():
                source.crop((0, top, width, bottom)).save(target)
            paths.append(target)
    return paths[0], paths[1]


def ocr_content_pages(book: str, model, processor, generate) -> None:
    config = BOOKS[book]
    import pymupdf

    root = book_dir(book)
    image_dir = WORK_DIR / "images" / book
    image_dir.mkdir(parents=True, exist_ok=True)
    doc = pymupdf.open(config["pdf"])
    # 普通单元的题页/答案页并不总是严格按固定页数排列；N3 后两个单元
    # 还会因详解长度出现额外页。用顶部标题识别页面类型比硬编码页表稳。
    for page in range(config["content_start"], config["mock_starts"][0]):
        image = image_dir / f"page{page:03d}.png"
        top_image = image_dir / f"page{page:03d}.top.png"
        top_output = root / f"page{page:03d}.top.txt"
        render_page(doc, page, image)
        if not top_image.exists():
            crop_top(image, top_image)
        if not top_output.exists() or not top_output.read_text(encoding="utf-8").strip():
            top_output.write_text(ocr(model, processor, generate, top_image, tokens=300, penalty=1.2).strip(), encoding="utf-8")
        top = top_output.read_text(encoding="utf-8")
        if book == "n1":
            is_answer = "問題" not in doc[page - 1].get_text("text")
        elif book == "n2":
            # 这两本普通单元固定为题页、答案页交替；顶部装饰会让 OCR 偶尔
            # 漏掉“解答”，页序本身更可靠。
            is_answer = (page - 7) % 2 == 1
        else:
            is_answer = bool(re.search(r"解答|答案|正解|解説", top))
        (root / f"page{page:03d}.q.txt").unlink(missing_ok=True) if is_answer else None
        (root / f"page{page:03d}.akey.txt").unlink(missing_ok=True) if not is_answer else None
        if is_answer:
            output = root / f"page{page:03d}.akey.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                output.write_text(top, encoding="utf-8")
            key_output = root / f"page{page:03d}.key.txt"
            if not key_output.exists() or not key_output.read_text(encoding="utf-8").strip():
                key_output.write_text(ocr_answer_key(model, processor, generate, top_image).strip(), encoding="utf-8")
            kind = "answer"
        else:
            output = root / f"page{page:03d}.q.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                full = ocr_question(model, processor, generate, image).strip()
                # 顶部 OCR 偶尔漏掉“解答”，从整页结果兜底纠正页面类型。
                if re.search(r"解答|答案|正解|解説", full[:500]) and not re.search(r"(?:^|\n)\s*問題", full[:500]):
                    (root / f"page{page:03d}.akey.txt").write_text(full, encoding="utf-8")
                    kind = "answer"
                else:
                    output.write_text(full, encoding="utf-8")
                    kind = "question"
            else:
                kind = "question"
        print(f"{book} content page {page} {kind}", flush=True)


def ocr_mocks(book: str, model, processor, generate) -> None:
    config = BOOKS[book]
    import pymupdf

    root = book_dir(book)
    image_dir = WORK_DIR / "images" / book
    image_dir.mkdir(parents=True, exist_ok=True)
    doc = pymupdf.open(config["pdf"])
    start = config["mock_starts"][0]
    for page in range(start, config["mock_end"] + 1):
        image = image_dir / f"page{page:03d}.png"
        top_image = image_dir / f"page{page:03d}.mocktop.png"
        top_output = root / f"page{page:03d}.mocktop.txt"
        render_page(doc, page, image)
        if not top_image.exists():
            crop_top(image, top_image)
        if not top_output.exists() or not top_output.read_text(encoding="utf-8").strip():
            top_output.write_text(ocr(model, processor, generate, top_image, tokens=300, penalty=1.2).strip(), encoding="utf-8")
        top = top_output.read_text(encoding="utf-8")
        fixed_kind = None
        if book == "n1":
            fixed_kind = "question" if any(s <= page < s + 7 for s in config["mock_starts"]) else "answer"
        elif book == "n2":
            fixed_kind = "question" if any(s <= page < s + 9 for s in config["mock_starts"]) else "answer"
        is_answer = fixed_kind == "answer" if fixed_kind else bool(re.search(r"解答|答案|正解|解説", top))
        (root / f"page{page:03d}.q.txt").unlink(missing_ok=True) if is_answer else None
        (root / f"page{page:03d}.a.txt").unlink(missing_ok=True) if not is_answer else None
        if is_answer:
            output = root / f"page{page:03d}.a.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                upper, lower = crop_halves(image, image_dir, page)
                upper_text = ocr(model, processor, generate, upper, tokens=1800, penalty=1.25)
                lower_text = ocr(model, processor, generate, lower, tokens=1800, penalty=1.25)
                output.write_text((upper_text + "\n" + lower_text).strip(), encoding="utf-8")
            kind = "answer"
        else:
            output = root / f"page{page:03d}.q.txt"
            if not output.exists() or not output.read_text(encoding="utf-8").strip():
                output.write_text(ocr_question(model, processor, generate, image).strip(), encoding="utf-8")
            kind = "question"
        print(f"{book} mock page {page} {kind}", flush=True)


def ocr_book(book: str) -> None:
    model, processor, generate = load_ocr_model()
    ocr_content_pages(book, model, processor, generate)
    ocr_mocks(book, model, processor, generate)


def option_matches(block: str) -> list[re.Match[str]]:
    return list(OPTION_RE.finditer(block))


def choose_question_matches(text: str, first: int, last: int) -> tuple[list[tuple[re.Match[str], re.Match[str] | None]], list[int]]:
    # 题号有时被 OCR 粘在段落中（尤其是文章填空题），按预期连续题号
    # 查找比“必须行首”更能容错；三位/四位题号不会与选项编号混淆。
    selected: list[re.Match[str]] = []
    missing: list[int] = []
    cursor = 0
    for number in range(first, last + 1):
        patterns = [re.compile(rf"(?<!\d)({number:03d})(?!\d)")]
        if number >= 100:
            patterns = [
                re.compile(rf"(?<!\d)(0*{number})(?!\d)"),
                re.compile(rf"(?m)^\s*[^\d\n]{{0,8}}(0*{number})"),
            ]
        else:
            patterns.append(re.compile(rf"(?m)^\s*({number})(?=\s|[.．、)])"))
        match = next((candidate.search(text, cursor) for candidate in patterns if candidate.search(text, cursor)), None)
        if match is None:
            missing.append(number)
            continue
        selected.append(match)
        cursor = match.end()
    selected_with_end = []
    for index, match in enumerate(selected):
        end = selected[index + 1] if index + 1 < len(selected) else None
        selected_with_end.append((match, end))
    return selected_with_end, missing


def parse_question_blocks(text: str, first: int, last: int, level: str, mock: bool) -> tuple[list[dict], list[int]]:
    text = normalize_text(text)
    selected, missing = choose_question_matches(text, first, last)
    items = []
    for match, next_match in selected:
        end = next_match.start() if next_match else len(text)
        block = text[match.end():end]
        options = option_matches(block)
        stem = compact(block[: options[0].start() if options else len(block)])
        option_values = []
        for index, option in enumerate(options[:4]):
            end_option = options[index + 1].start() if index + 1 < len(options) else len(block)
            value = compact(block[option.end():end_option])
            if value:
                option_values.append(value)
        number = int(match.group(1))
        anomalies = []
        if len(options) != 4 or len(option_values) != 4:
            anomalies.append("OCR 未识别出完整四选项，请人工核对原题")
        if len(stem) < 2:
            anomalies.append("OCR 题干过短，请人工核对原题")
        if NOISE_RE.search(stem) or any(NOISE_RE.search(x) for x in option_values):
            anomalies.append("题干或选项含疑似 OCR 噪声，请人工核对原题")
        section = None
        if mock:
            sections = list(SECTION_RE.finditer(text[:match.start()]))
            if sections:
                section = int(sections[-1].group(1))
            else:
                anomalies.append("模拟题小节未能从 OCR 文本识别，请人工核对科目")
        if mock:
            subject = "grammar" if section is not None and section >= 6 else "vocabulary"
        else:
            subject = "grammar" if (number - 1) % 6 >= 4 else "vocabulary"
        items.append({
            "number": number,
            "stem": stem,
            "options": [{"id": "abcd"[i], "label": "ABCD"[i], "text": option_values[i] if i < len(option_values) else ""}
                        for i in range(4)],
            "levelCode": level,
            "subjectCode": subject,
            "anomalies": anomalies,
        })
    return items, missing


def parse_unit_answers(root: Path) -> dict[int, int]:
    answers: dict[int, int] = {}
    paths = (list(root.glob("page*.akey.txt")) + list(root.glob("page*.key.txt"))
             + list(root.glob("page*.keytight.txt")))
    for path in sorted(paths, key=page_number):
        text = normalize_text(path.read_text(encoding="utf-8"))
        range_match = re.search(
            r"(\d{1,4})\s*[-~－一]\s*(\d{1,4})\s*(?:正解|正确|答案)?\s*[:：]?",
            text,
        )
        if not range_match:
            continue
        start, end = int(range_match.group(1)), int(range_match.group(2))
        after = text[range_match.end():].splitlines()[0] if text[range_match.end():] else ""
        digits = [int(x) for x in re.findall(r"(?<!\d)([1-4])(?!\d)", after)]
        if len(digits) != end - start + 1:
            nearby = text[range_match.end():range_match.end() + 100]
            digits = [int(x) for x in re.findall(r"(?<!\d)([1-4])(?!\d)", nearby)]
        for number, answer in zip(range(start, end + 1), digits):
            answers[number] = answer
    return answers


def parse_mock_answers(root: Path) -> dict[int, int]:
    answers: dict[int, int] = {}
    for path in sorted(root.glob("page*.a.txt"), key=page_number):
        text = normalize_text(path.read_text(encoding="utf-8"))
        for match in ANSWER_RE.finditer(text):
            number, answer = int(match.group(1)), int(match.group(2))
            if 1 <= number <= 1000:
                answers[number] = answer
    return answers


def question_page_text(path: Path, book: str, target_questions: int, pdf_doc=None) -> str:
    raw = path.read_text(encoding="utf-8")
    if book != "n1" or pdf_doc is None:
        return raw
    try:
        pdf_text = pdf_doc[page_number(path) - 1].get_text("text")
    except Exception:
        return raw

    def score(value: str) -> tuple[int, int, int]:
        normalized = normalize_text(value)
        q_count = len(re.findall(r"(?m)^\s*(?:[^\d\n]{0,8})\d{3,4}(?=\s|[.．、)])", normalized))
        option_count = len(option_matches(normalized))
        noise = len(NOISE_RE.findall(normalized))
        return (
            min(option_count, target_questions * 4) * 10 - abs(q_count - target_questions) * 12,
            -noise,
            -abs(q_count - target_questions),
        )

    return max((raw, pdf_text), key=score)


def merge_question_sources(primary: list[dict], secondary: list[dict], first: int, last: int) -> tuple[list[dict], list[int]]:
    by_number: dict[int, list[dict]] = {}
    for item in primary + secondary:
        by_number.setdefault(item["number"], []).append(item)

    def quality(item: dict) -> tuple[int, int, int, int]:
        texts = [item["stem"], *(option["text"] for option in item["options"])]
        filled = sum(bool(option["text"]) for option in item["options"])
        noise = len(NOISE_RE.findall(" ".join(texts)))
        return filled, -noise, -len(item["anomalies"]), min(len(item["stem"]), 200)

    merged = []
    missing = []
    for number in range(first, last + 1):
        candidates = by_number.get(number, [])
        if not candidates:
            missing.append(number)
            continue
        merged.append(max(candidates, key=quality))
    return merged, missing


def parse_question_sources(raw_text: str, pdf_text: str, first: int, last: int,
                          level: str, mock: bool) -> tuple[list[dict], list[int]]:
    primary, _ = parse_question_blocks(raw_text, first, last, level, mock)
    secondary = []
    if pdf_text.strip():
        secondary, _ = parse_question_blocks(pdf_text, first, last, level, mock)
    return merge_question_sources(primary, secondary, first, last)


def page_text_for_build(path: Path, pdf_doc=None) -> str:
    raw = path.read_text(encoding="utf-8")
    if pdf_doc is None or not re.search(r"(?m)^\s*#+\s*Question\s+\d+", raw):
        return clean_question_page_text(raw)
    pdf_text = pdf_doc[page_number(path) - 1].get_text("text")
    first_match = next(
        (
            match for match in re.finditer(r"(?m)^\s*(\d{3})(?=\D)", pdf_text)
            if int(match.group(1)) > 0
        ),
        None,
    )
    headings = list(re.finditer(r"(?m)^\s*(?:#+\s*)?Question\s+(\d+)\b", raw))
    if not first_match or not headings:
        return clean_question_page_text(raw)
    first = int(first_match.group(1))
    heading_first = int(headings[0].group(1))
    # 只修复 OCR 把整页题号重置成 1、2、3 的情况；PDF 文字层偶尔漏掉
    # 页首题号，不能据此覆盖本来正确的 OCR 题号。
    if heading_first < first and heading_first <= 10:
        replacements = {match.group(1): f"{first + index:03d}" for index, match in enumerate(headings[:6])}
        for local, number in replacements.items():
            raw = re.sub(rf"(?m)^(\s*(?:#+\s*)?)Question\s+{local}\b", rf"\g<1>{number}", raw, count=1)
    if len(headings) > 6 and "**Unit**" in raw:
        normalized_headings = list(re.finditer(r"(?m)^\s*###\s*(?:Question\s+)?\d+\s*$", raw))
        if len(normalized_headings) > 6:
            raw = raw[:normalized_headings[6].start()]
    return clean_question_page_text(raw)


def normalize_structured_question_text(text: str) -> str:
    """把 OCR 偶尔生成的 Markdown Question 块还原成普通四选题文本。"""
    headers = list(re.finditer(r"(?m)^\s*###\s*(\d{1,4})\s*$", text))
    if not headers or "**Question**" not in text:
        return text
    chunks = []
    cursor = 0
    for index, header in enumerate(headers):
        chunks.append(text[cursor:header.start()])
        end = headers[index + 1].start() if index + 1 < len(headers) else len(text)
        block = text[header.end():end]
        marker = re.search(r"(?m)^\s*-?\s*\*\*Question\*\*\s*:\s*", block)
        stem = None
        options_area = block
        if marker:
            after = block[marker.end():]
            lines = after.splitlines()
            first_line_index = next((i for i, line in enumerate(lines) if line.strip()), None)
            if first_line_index is not None:
                stem = re.sub(r"^1\s*[.．、)]\s*", "", lines[first_line_index].strip())
                options_area = "\n".join(lines[first_line_index + 1:])
            options_marker = re.search(r"(?m)^\s*-?\s*\*\*Options\*\*\s*:\s*", options_area)
            if options_marker:
                options_area = options_area[options_marker.end():]
        options = re.findall(
            r"(?m)^\s*(?:-\s*)?(?:\(\s*\)\s*)?([1-4])\s*[.．、:：)]\s*(.+?)\s*$",
            options_area,
        )
        if stem and len(options) == 4:
            chunks.append(f"### {header.group(1)}\n{stem}\n" + "\n".join(
                f"{number}. {value}" for number, value in options
            ) + "\n")
        else:
            chunks.append(text[header.start():end])
        cursor = end
    chunks.append(text[cursor:])
    return "".join(chunks)


def clean_question_page_text(text: str) -> str:
    """去掉 OCR 提示词/页眉页脚，避免污染相邻题目的最后一个选项。"""
    text = re.sub(r"(?im)^\s*(?:\d+\s+)?(?:the text|or paraphrase the text)[^\n]*\n?", "", text)
    text = re.sub(rf"\s+{OCR_PROMPT_MARKER_RE.pattern}[^\n]*", "", text, flags=re.I)
    text = re.sub(r"(?im)^(\s*(?:#+\s*)?)Question\s+(\d+)\b[^\n]*", r"\g<1>\2", text)
    text = re.split(r"(?im)^\s*#+\s*Image Description\b", text, maxsplit=1)[0]
    text = re.sub(r"(?i)[ \t]+(?:[-—]{2,}[ \t]*)?Unit\b[^\n]*", "", text)
    text = re.sub(r"(?i)[ \t]+(?:問題|问题)[ \t]*\d{2,4}\b[^\n]*", "", text)
    text = re.sub(r"(?i)[ \t]+\d{2,4}[ \t]*[-－][ \t]*\d{2,4}[ \t]*正解[^\n]*", "", text)
    text = re.sub(r"[ \t]+\d{3}[ \t]+\d{3}[ \t]*$", "", text, flags=re.M)
    text = re.sub(r"(?im)^\s*(?:[-—]{2,}\s*)?(?:[-*]\s*)?\*{0,2}Unit(?:\s+\d+)?\*{0,2}\s*$\n?", "", text)
    text = re.sub(r"(?im)^\s*(?:#+\s*)?(?:[-*]\s*)?\*{0,2}(?:文字|語彙|文法|問題|问题)\*{0,2}\s*$\n?", "", text)
    text = re.sub(r"(?m)^\s*[-—]{3,}\s*$\n?", "", text)
    return normalize_structured_question_text(text)


def build_item(parsed: dict, answer: int | None) -> dict:
    stem, dirty_stem = sanitize_ocr_fragment(parsed["stem"])
    anomalies = list(dict.fromkeys(parsed["anomalies"]))
    if dirty_stem:
        anomalies.append("题干含页眉或 OCR 提示词，已截去污染尾部")
    stem_placeholder = False
    if len(stem) < 2:
        stem = "题干 OCR 未能恢复，请人工核对原书"
        anomalies.append("题干过短或缺失，已写入待核对标记")
        stem_placeholder = True
    options = []
    incomplete_options = False
    dirty_option = False
    for option in parsed["options"][:4]:
        text, dirty = sanitize_ocr_fragment(option.get("text", ""))
        dirty_option = dirty_option or dirty
        if not text:
            text = "【待人工核对】"
            incomplete_options = True
        options.append({
            "id": option.get("id", ""),
            "label": option.get("label", ""),
            "text": text,
        })
    while len(options) < 4:
        index = len(options)
        options.append({"id": "abcd"[index], "label": "ABCD"[index], "text": "【待人工核对】"})
        incomplete_options = True
    if len(parsed["options"]) != 4:
        incomplete_options = True
    if len(options) == 4 and all(option["text"] and "待人工核对" not in option["text"] for option in options):
        anomalies = [anomaly for anomaly in anomalies if "完整四选项" not in anomaly]
    if incomplete_options:
        anomalies.append("选项不完整，缺失内容已写入待核对标记")
    if dirty_option:
        anomalies.append("选项含页眉或 OCR 提示词，已截去污染尾部")
    if answer is None:
        anomalies.append("答案页 OCR 未识别，请人工核对原书答案")
    if incomplete_options or len(parsed["stem"]) < 2 or stem_placeholder:
        answer = None
    item = {
        "rawExcerpt": stem[:5000],
        "materialKey": "",
        "type": "single_choice",
        "stem": stem,
        "options": options,
        "materialTitle": "",
        "materialContent": "",
        "levelCode": parsed["levelCode"],
        "subjectCode": parsed["subjectCode"],
        "difficulty": 3,
        "knowledgePointNames": [],
        "sourceAnswer": None if answer is None else {
            "value": {"optionIds": ["abcd"[answer - 1]]},
            "authority": "official",
            "explanation": "",
        },
        "aiSuggestedAnswer": None,
        "anomalies": anomalies,
    }
    return item


def validate_items(items: list[dict], level: str) -> list[str]:
    errors = []
    if len(items) != 1000:
        errors.append(f"题目数量为 {len(items)}，预期 1000")
    numbers = [int(x["number"]) for x in items]
    if numbers != list(range(1, 1001)):
        errors.append("题号不是 1 到 1000 的连续序列")
    for index, item in enumerate(items, 1):
        if set(item) - {"number"} != ITEM_KEYS:
            errors.append(f"第 {index} 题字段集合不符合导入契约")
        if item["levelCode"] != level or item["type"] != "single_choice":
            errors.append(f"第 {index} 题级别或题型不符合预期")
        if len(item["options"]) != 4 or any(not option["text"] for option in item["options"]):
            errors.append(f"第 {index} 题选项不完整")
        answer = item["sourceAnswer"]
        if answer is not None:
            ids = answer.get("value", {}).get("optionIds", [])
            if answer.get("authority") != "official" or len(ids) != 1 or ids[0] not in {"a", "b", "c", "d"}:
                errors.append(f"第 {index} 题答案结构不合法")
    return errors


def build_book(book: str) -> tuple[list[dict], list[str]]:
    config = BOOKS[book]
    root = book_dir(book)
    parsed_by_number: dict[int, dict] = {}
    errors: list[str] = []
    pdf_doc = None
    if book == "n1":
        import pymupdf
        pdf_doc = pymupdf.open(config["pdf"])
    q_paths = [
        path for path in sorted(root.glob("page*.q.txt"), key=page_number)
        if config["content_start"] <= page_number(path) < config["mock_starts"][0]
        and not (book == "n2" and page_number(path) == 221)
        and not (
            book == "n1"
            and (
                "解答" in pdf_doc[page_number(path) - 1].get_text("text")[:100]
                or re.search(r"\d{1,4}\s*[-~－]\s*\d{1,4}\s*正解", pdf_doc[page_number(path) - 1].get_text("text"))
                or re.search(r"(?i)(?:解答|正解|答案)", path.read_text(encoding="utf-8")[:400])
                or (
                    len(option_matches(pdf_doc[page_number(path) - 1].get_text("text"))) == 0
                    and len(option_matches(path.read_text(encoding="utf-8"))) < 20
                )
            )
        )
    ]
    q_text = "\n".join(
        page_text_for_build(path, pdf_doc)
        for path in q_paths
    )
    q_pdf_text = "\n".join(
        (clean_question_page_text(pdf_doc[page_number(path) - 1].get_text("text")) if pdf_doc is not None else "")
        for path in q_paths
    )
    q_start = 1
    for unit, count in enumerate(config["unit_counts"], 1):
        first, last = q_start, q_start + count - 1
        parsed, missing = parse_question_sources(q_text, q_pdf_text, first, last, config["level"], mock=False)
        manual_numbers = set(MANUAL_ITEMS_N2) if book == "n2" else set()
        missing = [number for number in missing if number not in manual_numbers]
        if missing:
            errors.append(f"unit {unit} 缺少题号: {missing[:12]}")
        parsed_by_number.update({item["number"]: item for item in parsed})
        q_start = last + 1
    mock_paths = [
        path for path in sorted(root.glob("page*.q.txt"), key=page_number)
        if page_number(path) >= config["mock_starts"][0]
    ]
    mock_q_text = "\n".join(page_text_for_build(path, pdf_doc) for path in mock_paths)
    mock_pdf_text = "\n".join(
        (clean_question_page_text(pdf_doc[page_number(path) - 1].get_text("text")) if pdf_doc is not None else "")
        for path in mock_paths
    )
    for first, last in config["mock_ranges"]:
        parsed, missing = parse_question_sources(mock_q_text, mock_pdf_text, first, last, config["level"], mock=True)
        manual_numbers = set(MANUAL_ITEMS_N1) if book == "n1" else set()
        if book == "n2":
            manual_numbers = set(MANUAL_ITEMS_N2)
        missing = [number for number in missing if number not in manual_numbers]
        if missing:
            errors.append(f"mock {first}-{last} 缺少题号: {missing[:12]}")
        parsed_by_number.update({item["number"]: item for item in parsed})
    if book == "n3":
        for number, option_texts in MANUAL_OPTION_FIXES_N3.items():
            if number not in parsed_by_number:
                continue
            parsed_by_number[number]["options"] = [
                {"id": "abcd"[index], "label": "ABCD"[index], "text": text}
                for index, text in enumerate(option_texts)
            ]
            parsed_by_number[number]["anomalies"] = [
                anomaly for anomaly in parsed_by_number[number]["anomalies"]
                if "完整四选项" not in anomaly
            ]
        for number, (stem, option_texts) in MANUAL_ITEMS_N3.items():
            parsed_by_number[number] = {
                "number": number,
                "stem": stem,
                "options": [
                    {"id": "abcd"[index], "label": "ABCD"[index], "text": text}
                    for index, text in enumerate(option_texts)
                ],
                "levelCode": config["level"],
                "subjectCode": "grammar" if number >= 882 or (number - 1) % 6 >= 4 else "vocabulary",
                "anomalies": [],
            }
    if book == "n1":
        for number, (stem, option_texts) in MANUAL_ITEMS_N1.items():
            parsed_by_number[number] = {
                "number": number,
                "stem": stem,
                "options": [
                    {"id": "abcd"[index], "label": "ABCD"[index], "text": text}
                    for index, text in enumerate(option_texts)
                ],
                "levelCode": config["level"],
                "subjectCode": "grammar",
                "anomalies": [],
            }
    if book == "n2":
        for number, (stem, option_texts) in MANUAL_ITEMS_N2.items():
            parsed_by_number[number] = {
                "number": number,
                "stem": stem,
                "options": [
                    {"id": "abcd"[index], "label": "ABCD"[index], "text": text}
                    for index, text in enumerate(option_texts)
                ],
                "levelCode": config["level"],
                "subjectCode": "vocabulary" if number in {643, 644, 662, 663, 664} else "grammar",
                "anomalies": [],
            }
    unit_answers = parse_unit_answers(root)
    mock_answers = parse_mock_answers(root)
    answers = {**unit_answers, **mock_answers}
    if book == "n1":
        answers.update(MANUAL_ANSWERS_N1)
    if book == "n2":
        answers.update(MANUAL_ANSWERS_N2)
    if book == "n3":
        answers.update(MANUAL_ANSWERS_N3)
    items = []
    for number in range(1, 1001):
        parsed = parsed_by_number.get(number)
        if parsed is None:
            errors.append(f"题号 {number} 缺少 OCR 内容，已生成待核对占位题")
            parsed = {
                "number": number,
                "stem": f"题号 {number}：OCR 未能恢复题干，请人工核对原书",
                "options": [
                    {"id": "abcd"[index], "label": "ABCD"[index], "text": ""}
                    for index in range(4)
                ],
                "levelCode": config["level"],
                "subjectCode": "grammar" if (number - 1) % 6 >= 4 else "vocabulary",
                "anomalies": ["整题 OCR 未能恢复，已生成待核对占位题"],
            }
        items.append({"number": number, **build_item(parsed, answers.get(number))})
    validation = validate_items(items, config["level"])
    errors.extend(validation)
    return items, list(dict.fromkeys(errors))


def write_book(book: str, items: list[dict]) -> None:
    config = BOOKS[book]
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    for path in OUT_DIR.glob(f"红蓝宝书1000题{book.upper()}-*.json"):
        path.unlink()
    for index in range(2):
        chunk = [{key: value for key, value in item.items() if key != "number"}
                 for item in items[index * 500:(index + 1) * 500]]
        target = OUT_DIR / f"红蓝宝书1000题{book.upper()}-{index + 1:02d}.json"
        target.write_text(json.dumps({"items": chunk}, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
        print(f"{target.name}: {len(chunk)}", flush=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--book", choices=sorted(BOOKS))
    parser.add_argument("--ocr-only", action="store_true")
    parser.add_argument("--build-only", action="store_true")
    args = parser.parse_args()
    books = [args.book] if args.book else list(BOOKS)
    if not args.build_only:
        for book in books:
            ocr_book(book)
    if not args.ocr_only:
        for book in books:
            items, errors = build_book(book)
            print(f"{book}: parsed={len(items)} answers={sum(item['sourceAnswer'] is not None for item in items)}", flush=True)
            if errors:
                print("WARNINGS", json.dumps(errors[:30], ensure_ascii=False), flush=True)
            write_book(book, items)


if __name__ == "__main__":
    main()
