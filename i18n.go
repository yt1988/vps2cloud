package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The languages of Me2Cloud's site, in the order the first-run choice lists them.
var languages = []struct{ Code, Name string }{
	{"en", "English"}, {"zh-CN", "简体中文"}, {"zh-TW", "繁體中文"}, {"ja", "日本語"}, {"ko", "한국어"},
	{"es", "Español"}, {"fr", "Français"}, {"de", "Deutsch"}, {"pt", "Português"}, {"ru", "Русский"},
}

// lang is the language of the menu and the human-readable output (JSON and CSV keys stay English).
var lang = "en"

// T looks up a text in the current language, falling back to English, and fills it like fmt.Sprintf.
func T(key string, a ...any) string {
	s, ok := texts[lang][key]
	if !ok {
		s = texts["en"][key]
	}
	if len(a) > 0 {
		return fmt.Sprintf(s, a...)
	}
	return s
}

// plural is the CLDR category of n in the current language: zh, ja and ko count with one form; en, de and es say
// "one" for 1, fr and pt for 0 and 1; ru has "one" (1, 21, 31 … but not 11), "few" (2–4, 22–24 …) and "many".
func plural(n int) string {
	switch lang {
	case "en", "de", "es":
		if n == 1 {
			return "one"
		}
	case "fr", "pt":
		if n == 0 || n == 1 {
			return "one"
		}
	case "ru":
		switch m10, m100 := n%10, n%100; {
		case m10 == 1 && m100 != 11:
			return "one"
		case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
			return "few"
		default:
			return "many"
		}
	}
	return "other"
}

// TN is T for a text that counts something: key_one, key_few or key_many when the language's rule for n names
// one and the language has that text, otherwise key. n is the counted number; a are all the arguments, n included.
func TN(key string, n int, a ...any) string {
	if p := plural(n); p != "other" {
		if _, ok := texts[lang][key+"_"+p]; ok {
			return T(key+"_"+p, a...)
		}
	}
	return T(key, a...)
}

func knownLang(code string) string {
	for _, l := range languages {
		if strings.EqualFold(l.Code, code) {
			return l.Code
		}
	}
	return ""
}

// The saved choice lives in the user's config directory (~/.config/vps2cloud/config.json on Linux).
type config struct {
	Lang      string   `json:"lang,omitempty"`
	Latest    string   `json:"latest,omitempty"`     // newest release seen
	CheckedAt int64    `json:"checked_at,omitempty"` // when it was looked up (Unix seconds)
	Last      []string `json:"last,omitempty"`       // the options of the last run, for the menu's first item
}

func configPath() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "vps2cloud", "config.json")
}

func loadConfig() config {
	var c config
	if p := configPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			json.Unmarshal(b, &c)
		}
	}
	return c
}

func saveConfig(c config) error {
	p := configPath()
	if p == "" {
		return fmt.Errorf("no config directory")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

var continentText = map[string]map[string]string{
	"en":    {"as": "Asia", "eu": "Europe", "na": "North America", "sa": "South America", "oc": "Oceania", "me": "Middle East", "af": "Africa"},
	"zh-CN": {"as": "亚洲", "eu": "欧洲", "na": "北美洲", "sa": "南美洲", "oc": "大洋洲", "me": "中东", "af": "非洲"},
	"zh-TW": {"as": "亞洲", "eu": "歐洲", "na": "北美洲", "sa": "南美洲", "oc": "大洋洲", "me": "中東", "af": "非洲"},
	"ja":    {"as": "アジア", "eu": "ヨーロッパ", "na": "北米", "sa": "南米", "oc": "オセアニア", "me": "中東", "af": "アフリカ"},
	"ko":    {"as": "아시아", "eu": "유럽", "na": "북미", "sa": "남미", "oc": "오세아니아", "me": "중동", "af": "아프리카"},
	"es":    {"as": "Asia", "eu": "Europa", "na": "Norteamérica", "sa": "Sudamérica", "oc": "Oceanía", "me": "Oriente Medio", "af": "África"},
	"fr":    {"as": "Asie", "eu": "Europe", "na": "Amérique du Nord", "sa": "Amérique du Sud", "oc": "Océanie", "me": "Moyen-Orient", "af": "Afrique"},
	"de":    {"as": "Asien", "eu": "Europa", "na": "Nordamerika", "sa": "Südamerika", "oc": "Ozeanien", "me": "Naher Osten", "af": "Afrika"},
	"pt":    {"as": "Ásia", "eu": "Europa", "na": "América do Norte", "sa": "América do Sul", "oc": "Oceania", "me": "Oriente Médio", "af": "África"},
	"ru":    {"as": "Азия", "eu": "Европа", "na": "Северная Америка", "sa": "Южная Америка", "oc": "Океания", "me": "Ближний Восток", "af": "Африка"},
}

func continentName(c string) string {
	if n, ok := continentText[lang][c]; ok {
		return n
	}
	return continentText["en"][c]
}

var texts = map[string]map[string]string{
	"en": {
		"title": "VPS2Cloud %s · latency from this machine to %d cloud regions", "machine": "This machine: %s · Enter keeps the [default] · Ctrl-C quits",
		"cdnItem": "CDN test: the nearest edge of %d CDNs", "noCDN": "this endpoint list has no CDNs.\n  Update vps2cloud, or run it without --offline.",
		"what": "What do you want to test?", "all": "All %d regions", "prov": "Choose providers", "cont": "Choose continents",
		"langItem": "Language", "choice": "Choice", "num": "Enter a number from 1 to %d.", "pickProv": "Providers (e.g. 1,3 or 2-5)", "pickCont": "Continents (e.g. 1,2)",
		"pickErr": "Enter one or more numbers from 1 to %d, like 1,3 or 2-5.", "ip": "Which IP version?", "v4": "IPv4", "v6": "IPv6 (regions without IPv6 are skipped)", "both": "Both in one run (one report, IPv4 and IPv6 parts)",
		"how": "How should it measure?", "tcp": "TCP handshake: just the network round trip", "https": "HTTPS: a request on an open connection, as an app sees it (slower)",
		"icmp": "ICMP ping (needs root)", "allModes": "All three, side by side", "samples": "Samples per region?", "quick": "Quick, 5", "std": "Standard, 10", "precise": "Precise, 20",
		"again": "To run the same test again without the menu:", "langPick": "Choose a language", "langSaved": "Language: %s. Change it in the menu or with --lang.",
		"m_tcp": "TCP handshake", "m_https": "HTTPS (reused connection)", "m_icmp": "ICMP ping", "nSamples": "%d samples", "from": "from", "unknownNet": "unknown network",
		"h_provider": "Provider", "h_region": "Region", "h_city": "City", "h_median": "Median", "h_min": "Min", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Loss", "h_tier": "Tier", "h_fit": "Fit",
		"more": "… %d more (--top 0 shows all)", "best": "Best region per provider", "noV6": " (%d more have no IPv6 address)",
		"link": "See and share:", "fullResult": "Full result", "topOf": "Top %d of %d regions.", "measuring": "measuring",
		"icmpSkip": "ICMP isn't available here (needs root or CAP_NET_RAW).\n  Measuring with the other methods.", "noMatch": "no region matches those filters",
		"back": "Back", "exit": "Exit", "answered": "%d regions of %d answered%s", "took": "Took %s · endpoint list: %s",
		"shortFail": "the short link service didn't answer, so here is the full link",
		"saved":     "Saved:", "saveQ": "Also save the result as a file?", "saveNo": "No, the table on screen is enough",
		"updAsk": "vps2cloud v%s is out (you have v%s). Update now?", "updNote": "vps2cloud v%s is out: run vps2cloud --update", "updDone": "Updated to v%s", "updLatest": "vps2cloud v%s is the latest version", "updFail": "update failed: %s", "updPerm": "no permission to replace %s.\n  Run it with sudo, or install again:\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "Run vps2cloud again to use the new version.",
		"partial":      "interrupted: %d regions of %d measured so far",
		"nSamples_one": "%d sample", "noV6_one": " (%d more has no IPv6 address)", "answered_one": "%d region of %d answered%s", "partial_one": "interrupted: %d region of %d measured so far", "fitOne_one": "%d region fits %s",
		"proxyWarn": "A local proxy or TUN seems to be answering: %s.\n  The numbers show the distance to it, not to the regions.", "proxyFake": "fake address %s", "proxyFast": "every continent under 5 ms", "proxyGap": "HTTPS over 50× slower than TCP",
		"bestSoFar": "best so far", "nearest": "Nearest: %s %s ms", "goodFor": "Good for: %s (--preset game marks them in the table)", "fitOne": "%d regions fit %s", "p_game": "gaming", "p_call": "video calls", "p_ssh": "SSH", "p_web": "websites",
		"lastItem": "Same as last time: %s", "badFlag": "no option --%s", "didYouMean": "did you mean --%s?", "seeHelp": "--help lists all options",
		"u_title": "VPS2Cloud: latency from this machine to %d regions of %d cloud providers.", "u_what": "What to test", "u_how": "How to measure", "u_out": "Output", "u_list": "Endpoint list",
		"u_exit": "Exit status:\n  0 done · 1 error · 2 bad options · 130 interrupted\n  3 the best median is above --max-ms",
	},
	"zh-CN": {
		"title": "VPS2Cloud %s · 测这台机器到 %d 个云区域的延迟", "machine": "本机：%s · 回车选 [默认] · Ctrl-C 退出",
		"cdnItem": "CDN 测试：%d 家 CDN 离这里最近的节点", "noCDN": "这份测速点清单里没有 CDN。\n  请更新 vps2cloud，或去掉 --offline 再运行。",
		"what": "要测什么？", "all": "全部 %d 个区域", "prov": "按厂商选", "cont": "按大洲选",
		"langItem": "语言", "choice": "选择", "num": "请输入 1 到 %d 之间的数字。", "pickProv": "厂商（如 1,3 或 2-5）", "pickCont": "大洲（如 1,2）",
		"pickErr": "请输入 1 到 %d 之间的一个或几个数字，比如 1,3 或 2-5。", "ip": "用哪个 IP 版本？", "v4": "IPv4", "v6": "IPv6（没有 IPv6 的区域会跳过）", "both": "两个一起测（一份报告，分 IPv4 和 IPv6 两部分）",
		"how": "怎么测？", "tcp": "TCP 握手：只测网络来回一趟的时间", "https": "HTTPS：在已建立的连接上发请求，和应用感受到的一样（慢一些）",
		"icmp": "ICMP ping（需要 root）", "allModes": "三种都测，并排对比", "samples": "每个区域测几次？", "quick": "快速，5 次", "std": "标准，10 次", "precise": "精确，20 次",
		"again": "下次跳过菜单，直接运行同样的测试：", "langPick": "选择语言", "langSaved": "语言：%s。之后可以在菜单里或用 --lang 修改。",
		"m_tcp": "TCP 握手", "m_https": "HTTPS（复用连接）", "m_icmp": "ICMP ping", "nSamples": "每个 %d 次", "from": "出口", "unknownNet": "未知网络",
		"h_provider": "厂商", "h_region": "区域", "h_city": "城市", "h_median": "中位数", "h_min": "最低", "h_p90": "P90", "h_jitter": "抖动", "h_loss": "丢包", "h_tier": "分档", "h_fit": "符合",
		"more": "… 还有 %d 个（加 --top 0 显示全部）", "best": "各厂商最快的区域", "noV6": "（另有 %d 个没有 IPv6 地址）",
		"link": "查看和分享：", "fullResult": "完整结果", "topOf": "共 %[2]d 个区域，这里显示最快的 %[1]d 个。", "measuring": "正在测",
		"icmpSkip": "这里用不了 ICMP（需要 root 或 CAP_NET_RAW）。\n  改用其他方式测。", "noMatch": "没有符合这些条件的区域",
		"back": "返回上一步", "exit": "退出", "answered": "%d / %d 个区域有回应%s", "took": "用时 %s · 测速点清单：%s",
		"shortFail": "短链接服务没有回应，下面是完整链接",
		"saved":     "已保存：", "saveQ": "要不要把结果也存成文件？", "saveNo": "不用，看屏幕上的表格就行",
		"updAsk": "有新版本 v%s（你现在用的是 v%s），现在更新吗？", "updNote": "vps2cloud 有新版本 v%s：运行 vps2cloud --update 更新", "updDone": "已更新到 v%s", "updLatest": "vps2cloud v%s 已经是最新版本", "updFail": "更新失败：%s", "updPerm": "没有权限替换 %s。\n  请用 sudo 运行，或者重新安装：\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "重新运行 vps2cloud 就能用上新版本。",
		"partial":   "已中断：下面是已经测完的 %d / %d 个区域",
		"proxyWarn": "这些连接像是本机的代理或 TUN 在应答：%s。\n  数字是到代理的距离，不是到云区域的。", "proxyFake": "解析到了假地址 %s", "proxyFast": "所有大洲都在 5 ms 以内", "proxyGap": "HTTPS 比 TCP 慢 50 倍以上",
		"bestSoFar": "目前最快", "nearest": "最近：%s %s ms", "goodFor": "适合：%s（加 --preset game 会在表里标出来）", "fitOne": "符合「%[2]s」的有 %[1]d 个区域", "p_game": "打游戏", "p_call": "视频会议", "p_ssh": "SSH", "p_web": "建站",
		"lastItem": "和上次一样：%s", "badFlag": "没有 --%s 这个选项", "didYouMean": "你是想用 --%s 吗？", "seeHelp": "--help 可以列出全部选项",
		"u_title": "VPS2Cloud：测这台机器到 %[2]d 家云厂商、%[1]d 个区域的延迟。", "u_what": "测什么", "u_how": "怎么测", "u_out": "输出", "u_list": "测速点清单",
		"u_exit": "退出码：\n  0 完成 · 1 出错 · 2 选项有误 · 130 被中断\n  3 最快的中位数超过了 --max-ms",
	},
	"zh-TW": {
		"title": "VPS2Cloud %s · 測這台機器到 %d 個雲端區域的延遲", "machine": "本機：%s · 按 Enter 選 [預設] · Ctrl-C 離開",
		"cdnItem": "CDN 測試：%d 家 CDN 離這裡最近的節點", "noCDN": "這份測速點清單裡沒有 CDN。\n  請更新 vps2cloud，或拿掉 --offline 再執行。",
		"what": "要測什麼？", "all": "全部 %d 個區域", "prov": "依業者選", "cont": "依大洲選",
		"langItem": "語言", "choice": "選擇", "num": "請輸入 1 到 %d 之間的數字。", "pickProv": "業者（例如 1,3 或 2-5）", "pickCont": "大洲（例如 1,2）",
		"pickErr": "請輸入 1 到 %d 之間的一個或幾個數字，例如 1,3 或 2-5。", "ip": "要用哪個 IP 版本？", "v4": "IPv4", "v6": "IPv6（沒有 IPv6 的區域會略過）", "both": "兩個一起測（一份報告，分 IPv4 和 IPv6 兩部分）",
		"how": "要怎麼測？", "tcp": "TCP 交握：只測網路來回一趟的時間", "https": "HTTPS：在已建立的連線上送請求，和應用程式感受到的一樣（比較慢）",
		"icmp": "ICMP ping（需要 root）", "allModes": "三種都測，並排比較", "samples": "每個區域測幾次？", "quick": "快速，5 次", "std": "標準，10 次", "precise": "精確，20 次",
		"again": "下次略過選單，直接執行同樣的測試：", "langPick": "選擇語言", "langSaved": "語言：%s。之後可以在選單裡或用 --lang 修改。",
		"m_tcp": "TCP 交握", "m_https": "HTTPS（沿用連線）", "m_icmp": "ICMP ping", "nSamples": "每個 %d 次", "from": "出口", "unknownNet": "不明網路",
		"h_provider": "業者", "h_region": "區域", "h_city": "城市", "h_median": "中位數", "h_min": "最低", "h_p90": "P90", "h_jitter": "抖動", "h_loss": "封包遺失", "h_tier": "分級", "h_fit": "符合",
		"more": "… 還有 %d 個（加 --top 0 顯示全部）", "best": "各業者最快的區域", "noV6": "（另有 %d 個沒有 IPv6 位址）",
		"link": "檢視和分享：", "fullResult": "完整結果", "topOf": "共 %[2]d 個區域，這裡顯示最快的 %[1]d 個。", "measuring": "正在測",
		"icmpSkip": "這裡無法使用 ICMP（需要 root 或 CAP_NET_RAW）。\n  改用其他方式測。", "noMatch": "沒有符合這些條件的區域",
		"back": "返回上一步", "exit": "離開", "answered": "%d / %d 個區域有回應%s", "took": "耗時 %s · 測速點清單：%s",
		"shortFail": "短網址服務沒有回應，下面是完整連結",
		"saved":     "已儲存：", "saveQ": "要不要把結果也存成檔案？", "saveNo": "不用，看螢幕上的表格就好",
		"updAsk": "有新版本 v%s（你目前用的是 v%s），現在更新嗎？", "updNote": "vps2cloud 有新版本 v%s：執行 vps2cloud --update 更新", "updDone": "已更新到 v%s", "updLatest": "vps2cloud v%s 已經是最新版本", "updFail": "更新失敗：%s", "updPerm": "沒有權限取代 %s。\n  請用 sudo 執行，或重新安裝：\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "重新執行 vps2cloud 就能用新版本。",
		"partial":   "已中斷：下面是已經測完的 %d / %d 個區域",
		"proxyWarn": "這些連線像是本機的代理或 TUN 在回應：%s。\n  數字是到代理的距離，不是到雲端區域的。", "proxyFake": "解析到假位址 %s", "proxyFast": "所有大洲都在 5 ms 以內", "proxyGap": "HTTPS 比 TCP 慢 50 倍以上",
		"bestSoFar": "目前最快", "nearest": "最近：%s %s ms", "goodFor": "適合：%s（加 --preset game 會在表裡標出來）", "fitOne": "符合「%[2]s」的有 %[1]d 個區域", "p_game": "玩遊戲", "p_call": "視訊會議", "p_ssh": "SSH", "p_web": "架站",
		"lastItem": "和上次一樣：%s", "badFlag": "沒有 --%s 這個選項", "didYouMean": "你是想用 --%s 嗎？", "seeHelp": "--help 可以列出全部選項",
		"u_title": "VPS2Cloud：測這台機器到 %[2]d 家雲端業者、%[1]d 個區域的延遲。", "u_what": "要測什麼", "u_how": "要怎麼測", "u_out": "輸出", "u_list": "測速點清單",
		"u_exit": "結束碼：\n  0 完成 · 1 出錯 · 2 選項有誤 · 130 被中斷\n  3 最快的中位數超過了 --max-ms",
	},
	"ja": {
		"title": "VPS2Cloud %s · このマシンから %d のクラウドリージョンまでの遅延", "machine": "このマシン：%s · Enter で [既定値] · Ctrl-C で終了",
		"cdnItem": "CDN テスト：%d 社の CDN のいちばん近い拠点", "noCDN": "この計測先の一覧には CDN がありません。\n  vps2cloud を更新するか、--offline を外して実行してください。",
		"what": "何を測りますか？", "all": "全 %d リージョン", "prov": "事業者を選ぶ", "cont": "大陸を選ぶ",
		"langItem": "言語", "choice": "選択", "num": "1〜%d の数字を入力してください。", "pickProv": "事業者（例：1,3 や 2-5）", "pickCont": "大陸（例：1,2）",
		"pickErr": "1〜%d の数字を 1 つ以上入力してください（例：1,3 や 2-5）。", "ip": "IP のバージョンは？", "v4": "IPv4", "v6": "IPv6（IPv6 のないリージョンは飛ばします）", "both": "両方を一度に（1 つのレポートに IPv4 と IPv6 を分けて表示）",
		"how": "どう測りますか？", "tcp": "TCP ハンドシェイク：ネットワークの往復時間だけ", "https": "HTTPS：開いた接続でリクエスト。アプリの体感に近い（時間がかかる）",
		"icmp": "ICMP ping（root が必要）", "allModes": "3 つすべてを並べて比べる", "samples": "1 リージョンあたり何回測りますか？", "quick": "クイック、5 回", "std": "標準、10 回", "precise": "精密、20 回",
		"again": "次回、メニューなしで同じテストを実行するには：", "langPick": "言語を選んでください", "langSaved": "言語：%s。メニューか --lang でいつでも変えられます。",
		"m_tcp": "TCP ハンドシェイク", "m_https": "HTTPS（接続を使い回し）", "m_icmp": "ICMP ping", "nSamples": "各 %d 回", "from": "出口", "unknownNet": "不明なネットワーク",
		"h_provider": "事業者", "h_region": "リージョン", "h_city": "都市", "h_median": "中央値", "h_min": "最小", "h_p90": "P90", "h_jitter": "ジッター", "h_loss": "ロス", "h_tier": "ランク", "h_fit": "適合",
		"more": "… ほか %d 件（--top 0 ですべて表示）", "best": "事業者ごとの最速リージョン", "noV6": "（ほか %d 件は IPv6 アドレスなし）",
		"link": "結果の表示と共有：", "fullResult": "すべての結果", "topOf": "%[2]d リージョンのうち上位 %[1]d 件。", "measuring": "計測中",
		"icmpSkip": "ここでは ICMP を使えません（root か CAP_NET_RAW が必要）。\n  ほかの方法で測ります。", "noMatch": "条件に合うリージョンがありません",
		"back": "前に戻る", "exit": "終了", "answered": "%d / %d リージョンが応答%s", "took": "所要 %s · 計測先の一覧：%s",
		"shortFail": "短縮リンクのサービスが応答しないため、元のリンクを表示します",
		"saved":     "保存しました：", "saveQ": "結果をファイルにも保存しますか？", "saveNo": "いいえ、画面の表で十分",
		"updAsk": "新しいバージョン v%s があります（いまは v%s）。更新しますか？", "updNote": "vps2cloud v%s が出ています：vps2cloud --update で更新", "updDone": "v%s に更新しました", "updLatest": "vps2cloud v%s は最新版です", "updFail": "更新できませんでした：%s", "updPerm": "%s を置き換える権限がありません。\n  sudo で実行するか、入れ直してください：\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "vps2cloud をもう一度実行すると、新しいバージョンになります。",
		"partial":   "中断しました：ここまでに測った %d / %d リージョン",
		"proxyWarn": "プロキシか TUN が応答しているようです：%s。\n  数値はプロキシまでの距離で、リージョンまでではありません。", "proxyFake": "偽のアドレス %s に解決", "proxyFast": "どの大陸も 5 ms 未満", "proxyGap": "HTTPS が TCP より 50 倍以上遅い",
		"bestSoFar": "現時点の最速", "nearest": "いちばん近い：%s %s ms", "goodFor": "向いている用途：%s（--preset game で表に印が付きます）", "fitOne": "「%[2]s」に合うリージョンは %[1]d 件", "p_game": "ゲーム", "p_call": "ビデオ会議", "p_ssh": "SSH", "p_web": "サイト",
		"lastItem": "前回と同じ：%s", "badFlag": "--%s というオプションはありません", "didYouMean": "--%s のことですか？", "seeHelp": "--help でオプションの一覧を表示",
		"u_title": "VPS2Cloud：このマシンから %[2]d 社のクラウド、%[1]d リージョンまでの遅延を測ります。", "u_what": "何を測るか", "u_how": "どう測るか", "u_out": "出力", "u_list": "計測先の一覧",
		"u_exit": "終了コード：\n  0 完了 · 1 エラー · 2 オプションの誤り · 130 中断\n  3 最速の中央値が --max-ms を超えた",
	},
	"ko": {
		"title": "VPS2Cloud %s · 이 머신에서 클라우드 리전 %d곳까지의 지연 시간", "machine": "이 머신: %s · Enter는 [기본값] · Ctrl-C는 종료",
		"cdnItem": "CDN 테스트: CDN %d곳의 가장 가까운 거점", "noCDN": "이 측정 지점 목록에는 CDN이 없어요.\n  vps2cloud를 업데이트하거나 --offline 없이 실행하세요.",
		"what": "무엇을 측정할까요?", "all": "전체 리전 %d곳", "prov": "업체 고르기", "cont": "대륙 고르기",
		"langItem": "언어", "choice": "선택", "num": "1부터 %d 사이의 숫자를 입력하세요.", "pickProv": "업체(예: 1,3 또는 2-5)", "pickCont": "대륙(예: 1,2)",
		"pickErr": "1부터 %d 사이의 숫자를 하나 이상 입력하세요(예: 1,3 또는 2-5).", "ip": "어떤 IP 버전으로 할까요?", "v4": "IPv4", "v6": "IPv6(IPv6가 없는 리전은 건너뜀)", "both": "둘 다 한 번에(보고서 하나에 IPv4·IPv6 따로)",
		"how": "어떻게 측정할까요?", "tcp": "TCP 핸드셰이크: 네트워크 왕복 시간만", "https": "HTTPS: 열린 연결로 요청, 앱이 느끼는 그대로(더 오래 걸림)",
		"icmp": "ICMP ping(root 필요)", "allModes": "세 가지 모두 나란히 비교", "samples": "리전마다 몇 번 측정할까요?", "quick": "빠르게, 5회", "std": "표준, 10회", "precise": "정밀, 20회",
		"again": "다음에 메뉴 없이 같은 테스트를 실행하려면:", "langPick": "언어를 고르세요", "langSaved": "언어: %s. 메뉴나 --lang으로 언제든 바꿀 수 있어요.",
		"m_tcp": "TCP 핸드셰이크", "m_https": "HTTPS(연결 재사용)", "m_icmp": "ICMP ping", "nSamples": "각 %d회", "from": "출구", "unknownNet": "알 수 없는 네트워크",
		"h_provider": "업체", "h_region": "리전", "h_city": "도시", "h_median": "중앙값", "h_min": "최소", "h_p90": "P90", "h_jitter": "지터", "h_loss": "손실", "h_tier": "등급", "h_fit": "적합",
		"more": "… %d곳 더(--top 0이면 전체)", "best": "업체별 가장 빠른 리전", "noV6": "(%d곳은 IPv6 주소 없음)",
		"link": "보기와 공유:", "fullResult": "전체 결과", "topOf": "리전 %[2]d곳 중 상위 %[1]d곳.", "measuring": "측정 중",
		"icmpSkip": "여기서는 ICMP를 쓸 수 없어요(root 또는 CAP_NET_RAW 필요).\n  다른 방법으로 측정해요.", "noMatch": "조건에 맞는 리전이 없어요",
		"back": "이전으로", "exit": "종료", "answered": "리전 %d / %d곳 응답%s", "took": "소요 %s · 측정 지점 목록: %s",
		"shortFail": "짧은 링크 서비스가 응답하지 않아 전체 링크를 보여 드려요",
		"saved":     "저장했어요:", "saveQ": "결과를 파일로도 저장할까요?", "saveNo": "아니요, 화면의 표면 충분해요",
		"updAsk": "새 버전 v%s 출시(지금은 v%s). 업데이트할까요?", "updNote": "vps2cloud v%s 출시: vps2cloud --update로 업데이트하세요", "updDone": "v%s 버전으로 업데이트했어요", "updLatest": "최신 버전(vps2cloud v%s)을 쓰고 있어요", "updFail": "업데이트 실패: %s", "updPerm": "%s 파일을 바꿀 권한이 없어요.\n  sudo로 실행하거나 다시 설치하세요:\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "vps2cloud를 다시 실행하면 새 버전이 적용돼요.",
		"partial":   "중단됨: 지금까지 측정한 리전 %d / %d곳",
		"proxyWarn": "로컬 프록시나 TUN이 응답하는 것 같아요: %s.\n  숫자는 프록시까지의 거리이지 리전까지가 아니에요.", "proxyFake": "가짜 주소 %s", "proxyFast": "모든 대륙이 5 ms 미만", "proxyGap": "HTTPS가 TCP보다 50배 이상 느림",
		"bestSoFar": "지금까지 최고", "nearest": "가장 가까운 곳: %s %s ms", "goodFor": "적합한 용도: %s(--preset game으로 표에 표시)", "fitOne": "'%[2]s'에 맞는 리전 %[1]d곳", "p_game": "게임", "p_call": "화상 회의", "p_ssh": "SSH", "p_web": "웹사이트",
		"lastItem": "지난번과 같게: %s", "badFlag": "--%s 옵션은 없어요", "didYouMean": "--%s 옵션을 말한 건가요?", "seeHelp": "--help로 옵션 목록 보기",
		"u_title": "VPS2Cloud: 이 머신에서 클라우드 업체 %[2]d곳, 리전 %[1]d곳까지의 지연 시간.", "u_what": "무엇을 측정할지", "u_how": "어떻게 측정할지", "u_out": "출력", "u_list": "측정 지점 목록",
		"u_exit": "종료 코드:\n  0 완료 · 1 오류 · 2 옵션 오류 · 130 중단\n  3 최고 중앙값이 --max-ms 초과",
	},
	"es": {
		"title": "VPS2Cloud %s · latencia de esta máquina a %d regiones de nube", "machine": "Esta máquina: %s · Enter deja el [valor por defecto] · Ctrl-C sale",
		"cdnItem": "Prueba de CDN: el nodo más cercano de %d CDN", "noCDN": "esta lista de puntos no tiene CDN.\n  Actualiza vps2cloud o ejecútalo sin --offline.",
		"what": "¿Qué quieres medir?", "all": "Las %d regiones", "prov": "Elegir proveedores", "cont": "Elegir continentes",
		"langItem": "Idioma", "choice": "Opción", "num": "Escribe un número del 1 al %d.", "pickProv": "Proveedores (p. ej. 1,3 o 2-5)", "pickCont": "Continentes (p. ej. 1,2)",
		"pickErr": "Escribe uno o más números del 1 al %d, como 1,3 o 2-5.", "ip": "¿Qué versión de IP?", "v4": "IPv4", "v6": "IPv6 (se omiten las regiones sin IPv6)", "both": "Las dos a la vez (un informe, con IPv4 e IPv6 por separado)",
		"how": "¿Cómo medir?", "tcp": "Handshake TCP: solo el viaje de ida y vuelta por la red", "https": "HTTPS: una petición en una conexión abierta, como la nota una app (más lento)",
		"icmp": "Ping ICMP (requiere root)", "allModes": "Las tres, una al lado de otra", "samples": "¿Cuántas mediciones por región?", "quick": "Rápida, 5", "std": "Estándar, 10", "precise": "Precisa, 20",
		"again": "Para repetir la misma prueba sin el menú:", "langPick": "Elige un idioma", "langSaved": "Idioma: %s. Puedes cambiarlo en el menú o con --lang.",
		"m_tcp": "Handshake TCP", "m_https": "HTTPS (conexión reutilizada)", "m_icmp": "Ping ICMP", "nSamples": "%d mediciones", "from": "desde", "unknownNet": "red desconocida",
		"h_provider": "Proveedor", "h_region": "Región", "h_city": "Ciudad", "h_median": "Mediana", "h_min": "Mín", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Pérdida", "h_tier": "Nivel", "h_fit": "Cumple",
		"more": "… %d más (--top 0 las muestra todas)", "best": "La mejor región de cada proveedor", "noV6": " (%d más sin dirección IPv6)",
		"link": "Ver y compartir:", "fullResult": "Resultado completo", "topOf": "Las %d mejores de %d regiones.", "measuring": "midiendo",
		"icmpSkip": "ICMP no está disponible aquí (requiere root o CAP_NET_RAW).\n  Se mide con los otros métodos.", "noMatch": "ninguna región cumple esos filtros",
		"back": "Atrás", "exit": "Salir", "answered": "%d regiones de %d respondieron%s", "took": "Tardó %s · lista de puntos: %s",
		"shortFail": "el servicio de enlaces cortos no respondió; este es el enlace completo",
		"saved":     "Guardado:", "saveQ": "¿Guardar también el resultado en un archivo?", "saveNo": "No, me basta con la tabla en pantalla",
		"updAsk": "Salió vps2cloud v%s (tienes v%s). ¿Actualizar ahora?", "updNote": "Salió vps2cloud v%s: ejecuta vps2cloud --update", "updDone": "Actualizado a v%s", "updLatest": "vps2cloud v%s es la última versión", "updFail": "no se pudo actualizar: %s", "updPerm": "sin permiso para reemplazar %s.\n  Ejecútalo con sudo o vuelve a instalar:\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "Vuelve a ejecutar vps2cloud para usar la versión nueva.",
		"partial":      "interrumpido: %d regiones de %d medidas hasta ahora",
		"nSamples_one": "%d medición", "answered_one": "%d región de %d respondió%s", "partial_one": "interrumpido: %d región de %d medida hasta ahora", "fitOne_one": "%d región sirve para %s",
		"proxyWarn": "Parece un proxy o TUN local: %s.\n  Son distancias al proxy, no a las regiones.", "proxyFake": "dirección falsa %s", "proxyFast": "todos los continentes por debajo de 5 ms", "proxyGap": "HTTPS más de 50× más lento que TCP",
		"bestSoFar": "la mejor hasta ahora", "nearest": "La más cercana: %s %s ms", "goodFor": "Sirve para: %s (--preset game las marca en la tabla)", "fitOne": "%d regiones sirven para %s", "p_game": "juegos", "p_call": "videollamadas", "p_ssh": "SSH", "p_web": "webs",
		"lastItem": "Igual que la última vez: %s", "badFlag": "no existe la opción --%s", "didYouMean": "¿quisiste decir --%s?", "seeHelp": "--help muestra todas las opciones",
		"u_title": "VPS2Cloud: latencia de esta máquina a %d regiones de %d proveedores de nube.", "u_what": "Qué medir", "u_how": "Cómo medir", "u_out": "Salida", "u_list": "Lista de puntos",
		"u_exit": "Código de salida:\n  0 listo · 1 error · 2 opciones mal · 130 interrumpido\n  3 la mejor mediana supera --max-ms",
	},
	"fr": {
		"title": "VPS2Cloud %s · latence de cette machine vers %d régions cloud", "machine": "Cette machine : %s · Entrée garde le [choix par défaut] · Ctrl-C quitte",
		"cdnItem": "Test CDN : le nœud le plus proche de %d CDN", "noCDN": "cette liste de points ne contient pas de CDN.\n  Mettez vps2cloud à jour ou lancez-le sans --offline.",
		"what": "Que voulez-vous mesurer ?", "all": "Les %d régions", "prov": "Choisir les fournisseurs", "cont": "Choisir les continents",
		"langItem": "Langue", "choice": "Choix", "num": "Entrez un nombre de 1 à %d.", "pickProv": "Fournisseurs (ex. 1,3 ou 2-5)", "pickCont": "Continents (ex. 1,2)",
		"pickErr": "Entrez un ou plusieurs nombres de 1 à %d, comme 1,3 ou 2-5.", "ip": "Quelle version d’IP ?", "v4": "IPv4", "v6": "IPv6 (les régions sans IPv6 sont ignorées)", "both": "Les deux d’un coup (un rapport, parties IPv4 et IPv6)",
		"how": "Comment mesurer ?", "tcp": "Poignée de main TCP : l’aller-retour réseau seul", "https": "HTTPS : une requête sur une connexion ouverte, comme une appli (plus long)",
		"icmp": "Ping ICMP (nécessite root)", "allModes": "Les trois côte à côte", "samples": "Combien de mesures par région ?", "quick": "Rapide, 5", "std": "Standard, 10", "precise": "Précis, 20",
		"again": "Pour relancer le même test sans le menu :", "langPick": "Choisissez une langue", "langSaved": "Langue : %s. Modifiable dans le menu ou avec --lang.",
		"m_tcp": "Poignée de main TCP", "m_https": "HTTPS (connexion réutilisée)", "m_icmp": "Ping ICMP", "nSamples": "%d mesures", "from": "depuis", "unknownNet": "réseau inconnu",
		"h_provider": "Fournisseur", "h_region": "Région", "h_city": "Ville", "h_median": "Médiane", "h_min": "Min", "h_p90": "P90", "h_jitter": "Gigue", "h_loss": "Perte", "h_tier": "Niveau", "h_fit": "OK",
		"more": "… %d de plus (--top 0 affiche tout)", "best": "Meilleure région par fournisseur", "noV6": " (%d autres n’ont pas d’adresse IPv6)",
		"link": "Voir et partager :", "fullResult": "Résultat complet", "topOf": "Les %d meilleures sur %d régions.", "measuring": "mesure en cours",
		"icmpSkip": "ICMP n’est pas disponible ici (root ou CAP_NET_RAW requis).\n  Mesure avec les autres méthodes.", "noMatch": "aucune région ne correspond à ces filtres",
		"back": "Retour", "exit": "Quitter", "answered": "%d régions sur %d ont répondu%s", "took": "Durée %s · liste des points : %s",
		"shortFail": "le service de liens courts n’a pas répondu ; voici le lien complet",
		"saved":     "Enregistré :", "saveQ": "Enregistrer aussi le résultat dans un fichier ?", "saveNo": "Non, le tableau à l’écran suffit",
		"updAsk": "vps2cloud v%s est sorti (vous avez v%s). Mettre à jour ?", "updNote": "vps2cloud v%s est sorti : lancez vps2cloud --update", "updDone": "Mis à jour en v%s", "updLatest": "vps2cloud v%s est la dernière version", "updFail": "échec de la mise à jour : %s", "updPerm": "pas le droit de remplacer %s.\n  Lancez-le avec sudo ou réinstallez :\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "Relancez vps2cloud pour utiliser la nouvelle version.",
		"partial":      "interrompu : voici %d régions mesurées sur %d",
		"nSamples_one": "%d mesure", "noV6_one": " (%d autre n’a pas d’adresse IPv6)", "answered_one": "%d région sur %d a répondu%s", "partial_one": "interrompu : voici %d région mesurée sur %d", "fitOne_one": "%d région convient pour %s",
		"proxyWarn": "Un proxy ou TUN local semble répondre : %s.\n  Ce sont les distances au proxy, pas aux régions.", "proxyFake": "fausse adresse %s", "proxyFast": "tous les continents sous 5 ms", "proxyGap": "HTTPS plus de 50× plus lent que TCP",
		"bestSoFar": "meilleure pour l’instant", "nearest": "La plus proche : %s %s ms", "goodFor": "Convient pour : %s (--preset game les marque dans le tableau)", "fitOne": "%d régions conviennent pour %s", "p_game": "le jeu", "p_call": "la visio", "p_ssh": "SSH", "p_web": "un site web",
		"lastItem": "Comme la dernière fois : %s", "badFlag": "l’option --%s n’existe pas", "didYouMean": "vouliez-vous dire --%s ?", "seeHelp": "--help liste toutes les options",
		"u_title": "VPS2Cloud : latence de cette machine vers %d régions de %d fournisseurs cloud.", "u_what": "Que mesurer", "u_how": "Comment mesurer", "u_out": "Sortie", "u_list": "Liste des points",
		"u_exit": "Code de sortie :\n  0 terminé · 1 erreur · 2 options invalides · 130 interrompu\n  3 meilleure médiane au-delà de --max-ms",
	},
	"de": {
		"title": "VPS2Cloud %s · Latenz von diesem Rechner zu %d Cloud-Regionen", "machine": "Dieser Rechner: %s · Enter nimmt den [Standard] · Strg-C beendet",
		"cdnItem": "CDN-Test: der nächste Knoten von %d CDNs", "noCDN": "diese Messpunktliste enthält keine CDNs.\n  vps2cloud aktualisieren oder ohne --offline starten.",
		"what": "Was willst du messen?", "all": "Alle %d Regionen", "prov": "Anbieter wählen", "cont": "Kontinente wählen",
		"langItem": "Sprache", "choice": "Auswahl", "num": "Gib eine Zahl von 1 bis %d ein.", "pickProv": "Anbieter (z. B. 1,3 oder 2-5)", "pickCont": "Kontinente (z. B. 1,2)",
		"pickErr": "Gib eine oder mehrere Zahlen von 1 bis %d ein, etwa 1,3 oder 2-5.", "ip": "Welche IP-Version?", "v4": "IPv4", "v6": "IPv6 (Regionen ohne IPv6 werden übersprungen)", "both": "Beide in einem Lauf (ein Bericht mit IPv4- und IPv6-Teil)",
		"how": "Wie messen?", "tcp": "TCP-Handshake: nur die Umlaufzeit im Netz", "https": "HTTPS: Anfrage über eine offene Verbindung, wie eine App (dauert länger)",
		"icmp": "ICMP-Ping (braucht root)", "allModes": "Alle drei nebeneinander", "samples": "Wie viele Messungen pro Region?", "quick": "Schnell, 5", "std": "Standard, 10", "precise": "Genau, 20",
		"again": "Denselben Test ohne Menü noch mal starten:", "langPick": "Sprache wählen", "langSaved": "Sprache: %s. Änderbar im Menü oder mit --lang.",
		"m_tcp": "TCP-Handshake", "m_https": "HTTPS (Verbindung wiederverwendet)", "m_icmp": "ICMP-Ping", "nSamples": "%d Messungen", "from": "von", "unknownNet": "unbekanntes Netz",
		"h_provider": "Anbieter", "h_region": "Region", "h_city": "Stadt", "h_median": "Median", "h_min": "Min", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Verlust", "h_tier": "Stufe", "h_fit": "Passt",
		"more": "… %d weitere (--top 0 zeigt alle)", "best": "Beste Region je Anbieter", "noV6": " (%d weitere ohne IPv6-Adresse)",
		"link": "Ansehen und teilen:", "fullResult": "Vollständiges Ergebnis", "topOf": "Die besten %d von %d Regionen.", "measuring": "Messung läuft",
		"icmpSkip": "ICMP geht hier nicht (braucht root oder CAP_NET_RAW).\n  Gemessen wird mit den anderen Methoden.", "noMatch": "keine Region passt zu diesen Filtern",
		"back": "Zurück", "exit": "Beenden", "answered": "%d Regionen von %d antworteten%s", "took": "Dauer %s · Messpunktliste: %s",
		"shortFail": "der Kurzlink-Dienst hat nicht geantwortet; hier der vollständige Link",
		"saved":     "Gespeichert:", "saveQ": "Ergebnis zusätzlich als Datei speichern?", "saveNo": "Nein, die Tabelle auf dem Bildschirm reicht",
		"updAsk": "vps2cloud v%s ist da (du hast v%s). Jetzt aktualisieren?", "updNote": "vps2cloud v%s ist da: vps2cloud --update ausführen", "updDone": "Auf v%s aktualisiert", "updLatest": "vps2cloud v%s ist die neueste Version", "updFail": "Aktualisierung fehlgeschlagen: %s", "updPerm": "keine Berechtigung, %s zu ersetzen.\n  Mit sudo starten oder neu installieren:\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "Starte vps2cloud neu, um die neue Version zu nutzen.",
		"partial":      "abgebrochen: %d Regionen von %d bisher gemessen",
		"nSamples_one": "%d Messung", "answered_one": "%d Region von %d antwortete%s", "partial_one": "abgebrochen: %d Region von %d bisher gemessen", "fitOne_one": "%d Region passt für %s",
		"proxyWarn": "Ein lokaler Proxy oder TUN antwortet wohl: %s.\n  Das ist der Weg zum Proxy, nicht zu den Regionen.", "proxyFake": "falsche Adresse %s", "proxyFast": "alle Kontinente unter 5 ms", "proxyGap": "HTTPS über 50× langsamer als TCP",
		"bestSoFar": "bisher am besten", "nearest": "Am nächsten: %s %s ms", "goodFor": "Geeignet für: %s (--preset game markiert sie in der Tabelle)", "fitOne": "%d Regionen passen für %s", "p_game": "Gaming", "p_call": "Videocalls", "p_ssh": "SSH", "p_web": "Websites",
		"lastItem": "Wie beim letzten Mal: %s", "badFlag": "die Option --%s gibt es nicht", "didYouMean": "meintest du --%s?", "seeHelp": "--help zeigt alle Optionen",
		"u_title": "VPS2Cloud: Latenz von diesem Rechner zu %d Regionen von %d Cloud-Anbietern.", "u_what": "Was messen", "u_how": "Wie messen", "u_out": "Ausgabe", "u_list": "Messpunktliste",
		"u_exit": "Exit-Status:\n  0 fertig · 1 Fehler · 2 falsche Optionen · 130 abgebrochen\n  3 bester Median über --max-ms",
	},
	"pt": {
		"title": "VPS2Cloud %s · latência desta máquina até %d regiões de nuvem", "machine": "Esta máquina: %s · Enter mantém o [padrão] · Ctrl-C sai",
		"cdnItem": "Teste de CDN: o nó mais próximo de %d CDNs", "noCDN": "esta lista de pontos não tem CDNs.\n  Atualize o vps2cloud ou rode sem --offline.",
		"what": "O que você quer medir?", "all": "Todas as %d regiões", "prov": "Escolher provedores", "cont": "Escolher continentes",
		"langItem": "Idioma", "choice": "Opção", "num": "Digite um número de 1 a %d.", "pickProv": "Provedores (ex. 1,3 ou 2-5)", "pickCont": "Continentes (ex. 1,2)",
		"pickErr": "Digite um ou mais números de 1 a %d, como 1,3 ou 2-5.", "ip": "Qual versão de IP?", "v4": "IPv4", "v6": "IPv6 (regiões sem IPv6 ficam de fora)", "both": "As duas de uma vez (um relatório, IPv4 e IPv6 separados)",
		"how": "Como medir?", "tcp": "Handshake TCP: só a ida e volta pela rede", "https": "HTTPS: uma requisição numa conexão aberta, como um app sente (mais lento)",
		"icmp": "Ping ICMP (requer root)", "allModes": "As três lado a lado", "samples": "Quantas medições por região?", "quick": "Rápido, 5", "std": "Padrão, 10", "precise": "Preciso, 20",
		"again": "Para repetir o mesmo teste sem o menu:", "langPick": "Escolha um idioma", "langSaved": "Idioma: %s. Dá para trocar no menu ou com --lang.",
		"m_tcp": "Handshake TCP", "m_https": "HTTPS (conexão reaproveitada)", "m_icmp": "Ping ICMP", "nSamples": "%d medições", "from": "de", "unknownNet": "rede desconhecida",
		"h_provider": "Provedor", "h_region": "Região", "h_city": "Cidade", "h_median": "Mediana", "h_min": "Mín", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Perda", "h_tier": "Nível", "h_fit": "Cabe",
		"more": "… mais %d (--top 0 mostra todas)", "best": "Melhor região de cada provedor", "noV6": " (mais %d sem endereço IPv6)",
		"link": "Ver e compartilhar:", "fullResult": "Resultado completo", "topOf": "As %d melhores de %d regiões.", "measuring": "medindo",
		"icmpSkip": "ICMP não está disponível aqui (requer root ou CAP_NET_RAW).\n  Medindo pelos outros métodos.", "noMatch": "nenhuma região bate com esses filtros",
		"back": "Voltar", "exit": "Sair", "answered": "%d regiões de %d responderam%s", "took": "Levou %s · lista de pontos: %s",
		"shortFail": "o serviço de links curtos não respondeu; este é o link completo",
		"saved":     "Salvo:", "saveQ": "Salvar o resultado também num arquivo?", "saveNo": "Não, a tabela na tela basta",
		"updAsk": "Saiu o vps2cloud v%s (você tem o v%s). Atualizar agora?", "updNote": "Saiu o vps2cloud v%s: rode vps2cloud --update", "updDone": "Atualizado para v%s", "updLatest": "vps2cloud v%s é a versão mais recente", "updFail": "falha ao atualizar: %s", "updPerm": "sem permissão para substituir %s.\n  Rode com sudo ou instale de novo:\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "Rode o vps2cloud de novo para usar a versão nova.",
		"partial":      "interrompido: %d regiões de %d medidas até agora",
		"nSamples_one": "%d medição", "answered_one": "%d região de %d respondeu%s", "partial_one": "interrompido: %d região de %d medida até agora", "fitOne_one": "%d região serve para %s",
		"proxyWarn": "Parece um proxy ou TUN local: %s.\n  São distâncias até o proxy, não até as regiões.", "proxyFake": "endereço falso %s", "proxyFast": "todos os continentes abaixo de 5 ms", "proxyGap": "HTTPS mais de 50× mais lento que TCP",
		"bestSoFar": "melhor até agora", "nearest": "Mais próxima: %s %s ms", "goodFor": "Serve para: %s (--preset game marca na tabela)", "fitOne": "%d regiões servem para %s", "p_game": "jogos", "p_call": "videochamadas", "p_ssh": "SSH", "p_web": "sites",
		"lastItem": "Igual à última vez: %s", "badFlag": "não existe a opção --%s", "didYouMean": "você quis dizer --%s?", "seeHelp": "--help mostra todas as opções",
		"u_title": "VPS2Cloud: latência desta máquina até %d regiões de %d provedores de nuvem.", "u_what": "O que medir", "u_how": "Como medir", "u_out": "Saída", "u_list": "Lista de pontos",
		"u_exit": "Código de saída:\n  0 concluído · 1 erro · 2 opções inválidas · 130 interrompido\n  3 melhor mediana acima de --max-ms",
	},
	"ru": {
		"title": "VPS2Cloud %s · задержка от этой машины до %d облачных регионов", "machine": "Эта машина: %s · Enter — [по умолчанию] · Ctrl-C — выход",
		"cdnItem": "Тест CDN: ближайший узел %d CDN", "noCDN": "в этом списке точек замера нет CDN.\n  Обновите vps2cloud или запустите без --offline.",
		"what": "Что будем мерить?", "all": "Все %d регионов", "prov": "Выбрать провайдеров", "cont": "Выбрать континенты",
		"langItem": "Язык", "choice": "Выбор", "num": "Введите число от 1 до %d.", "pickProv": "Провайдеры (напр. 1,3 или 2-5)", "pickCont": "Континенты (напр. 1,2)",
		"pickErr": "Введите одно или несколько чисел от 1 до %d, например 1,3 или 2-5.", "ip": "Какая версия IP?", "v4": "IPv4", "v6": "IPv6 (регионы без IPv6 пропускаются)", "both": "Обе за один запуск (один отчёт, IPv4 и IPv6 отдельно)",
		"how": "Как мерить?", "tcp": "TCP-рукопожатие: только путь по сети туда и обратно", "https": "HTTPS: запрос по открытому соединению, как у приложения (дольше)",
		"icmp": "ICMP-пинг (нужен root)", "allModes": "Все три рядом", "samples": "Сколько замеров на регион?", "quick": "Быстро, 5", "std": "Стандарт, 10", "precise": "Точно, 20",
		"again": "Чтобы повторить тот же тест без меню:", "langPick": "Выберите язык", "langSaved": "Язык: %s. Сменить можно в меню или через --lang.",
		"m_tcp": "TCP-рукопожатие", "m_https": "HTTPS (то же соединение)", "m_icmp": "ICMP-пинг", "nSamples": "%d замеров", "from": "откуда", "unknownNet": "неизвестная сеть",
		"h_provider": "Провайдер", "h_region": "Регион", "h_city": "Город", "h_median": "Медиана", "h_min": "Мин", "h_p90": "P90", "h_jitter": "Джиттер", "h_loss": "Потери", "h_tier": "Уровень", "h_fit": "Подходит",
		"more": "… ещё %d (--top 0 покажет все)", "best": "Лучший регион каждого провайдера", "noV6": " (ещё у %d нет IPv6-адреса)",
		"link": "Посмотреть и поделиться:", "fullResult": "Полный результат", "topOf": "Лучшие %d из %d регионов.", "measuring": "идёт замер",
		"icmpSkip": "ICMP здесь недоступен (нужен root или CAP_NET_RAW).\n  Меряем другими способами.", "noMatch": "под эти фильтры не подходит ни один регион",
		"back": "Назад", "exit": "Выход", "answered": "%d регионов из %d ответили%s", "took": "Заняло %s · список точек: %s",
		"shortFail": "сервис коротких ссылок не ответил; вот полная ссылка",
		"saved":     "Сохранено:", "saveQ": "Сохранить результат ещё и в файл?", "saveNo": "Нет, хватит таблицы на экране",
		"updAsk": "Вышла vps2cloud v%s (у вас v%s). Обновить сейчас?", "updNote": "Вышла vps2cloud v%s: выполните vps2cloud --update", "updDone": "Обновлено до v%s", "updLatest": "vps2cloud v%s — последняя версия", "updFail": "не удалось обновить: %s", "updPerm": "нет прав заменить %s.\n  Запустите через sudo или переустановите:\n  curl -fsSL https://m2c.1ms.dog/i | sh", "updRestart": "Запустите vps2cloud снова, чтобы перейти на новую версию.",
		"partial":      "прервано: %d регионов из %d измерено к этому моменту",
		"nSamples_one": "%d замер", "nSamples_few": "%d замера", "answered_one": "%d регион из %d ответил%s", "answered_few": "%d региона из %d ответили%s",
		"partial_one": "прервано: %d регион из %d измерен к этому моменту", "partial_few": "прервано: %d региона из %d измерено к этому моменту",
		"fitOne_one": "Подходят для «%[2]s»: %[1]d регион", "fitOne_few": "Подходят для «%[2]s»: %[1]d региона",
		"all_one": "Все %d регион", "all_few": "Все %d региона", "title_one": "VPS2Cloud %s · задержка от этой машины до %d облачного региона",
		"proxyWarn": "Похоже, отвечает локальный прокси или TUN: %s.\n  Это расстояние до прокси, а не до регионов.", "proxyFake": "поддельный адрес %s", "proxyFast": "все континенты быстрее 5 мс", "proxyGap": "HTTPS в 50+ раз медленнее TCP",
		"bestSoFar": "лучший пока", "nearest": "Ближайший: %s %s мс", "goodFor": "Подходит под задачи: %s (--preset game отметит их в таблице)", "fitOne": "Подходят для «%[2]s»: %[1]d регионов", "p_game": "игры", "p_call": "видеозвонки", "p_ssh": "SSH", "p_web": "сайты",
		"lastItem": "Как в прошлый раз: %s", "badFlag": "опции --%s нет", "didYouMean": "вы имели в виду --%s?", "seeHelp": "--help покажет все опции",
		"u_title": "VPS2Cloud: задержка от этой машины до %d регионов %d облачных провайдеров.", "u_what": "Что мерить", "u_how": "Как мерить", "u_out": "Вывод", "u_list": "Список точек замера",
		"u_exit": "Код выхода:\n  0 готово · 1 ошибка · 2 неверные опции · 130 прервано\n  3 лучшая медиана выше --max-ms",
	},
}
