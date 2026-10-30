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
	Lang      string `json:"lang,omitempty"`
	Latest    string `json:"latest,omitempty"`     // newest release seen
	CheckedAt int64  `json:"checked_at,omitempty"` // when it was looked up (Unix seconds)
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
		"what": "What to test?", "all": "All %d regions", "prov": "Choose providers", "cont": "Choose continents",
		"langItem": "Language", "choice": "Choice", "num": "Enter a number from 1 to %d.", "pickProv": "Providers (e.g. 1,3 or 2-5)", "pickCont": "Continents (e.g. 1,2)",
		"pickErr": "Enter one or more numbers from 1 to %d, like 1,3 or 2-5.", "ip": "IP version?", "v4": "IPv4", "v6": "IPv6 (regions without IPv6 are skipped)", "both": "Both in one run (one report, IPv4 and IPv6 sections)",
		"how": "How to measure?", "tcp": "TCP handshake: the network round trip alone", "https": "HTTPS: a request on an open connection, as an app sees it (slower to run)",
		"icmp": "ICMP ping (needs root)", "allModes": "All three side by side", "samples": "Samples per region?", "quick": "Quick, 5", "std": "Standard, 10", "precise": "Precise, 20",
		"again": "To run the same test without the menu:", "langPick": "Choose a language", "langSaved": "Language: %s. Change it from the menu or with --lang.",
		"m_tcp": "TCP handshake", "m_https": "HTTPS (reused connection)", "m_icmp": "ICMP echo", "nSamples": "%d samples", "from": "from", "unknownNet": "unknown network",
		"h_provider": "Provider", "h_region": "Region", "h_city": "City", "h_median": "Median", "h_min": "Min", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Loss", "h_tier": "Tier", "h_fit": "Fit",
		"more": "… %d more (--top 0 shows all)", "best": "Best per provider", "summary": "%d of %d regions answered%s in %s · endpoints: %s", "noV6": " (%d more have no IPv6 address)",
		"link": "View and share:", "fullResult": "Full result", "topOf": "Top %d of %d regions.", "measuring": "measuring",
		"icmpSkip": "ICMP is not available here (needs root or CAP_NET_RAW); measuring the other modes", "noMatch": "no region matches those filters",
		"back": "Back", "exit": "Exit", "answered": "%d of %d regions answered%s", "took": "Took %s · endpoints: %s",
		"shortFail": "the short link service did not answer; showing the full link",
		"saved":     "Saved:", "saveQ": "Also save the result as a file?", "saveNo": "No, the table on screen is enough",
		"updAsk": "vps2cloud v%s is out (you have v%s). Update now?", "updNote": "vps2cloud v%s is available: run vps2cloud --update", "updDone": "Updated to v%s", "updLatest": "vps2cloud v%s is the latest version", "updFail": "update failed: %s", "updPerm": "no permission to replace %s; run it with sudo, or install again: curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "Run vps2cloud again to use the new version.",
		"partial": "interrupted: the %d of %d regions measured so far",
	},
	"zh-CN": {
		"title": "VPS2Cloud %s · 测这台机器到 %d 个云区域的延迟", "machine": "本机：%s · 回车选 [默认] · Ctrl-C 退出",
		"what": "测什么？", "all": "全部 %d 个区域", "prov": "按厂商选", "cont": "按大洲选",
		"langItem": "语言", "choice": "选择", "num": "请输入 1 到 %d 之间的数字。", "pickProv": "厂商（如 1,3 或 2-5）", "pickCont": "大洲（如 1,2）",
		"pickErr": "请输入 1 到 %d 之间的一个或多个数字，如 1,3 或 2-5。", "ip": "IP 版本？", "v4": "IPv4", "v6": "IPv6（没有 IPv6 的区域会跳过）", "both": "两个一起测（一份报告，分 IPv4 和 IPv6 两部分）",
		"how": "怎么测？", "tcp": "TCP 握手：只测网络往返", "https": "HTTPS：在已建立的连接上发请求，和应用看到的一样（耗时更长）",
		"icmp": "ICMP ping（需要 root）", "allModes": "三种都测，并排对比", "samples": "每个区域采样几次？", "quick": "快速，5 次", "std": "标准，10 次", "precise": "精确，20 次",
		"again": "下次不用菜单，直接运行同样的测试：", "langPick": "选择语言", "langSaved": "语言：%s。之后可以在菜单里或用 --lang 修改。",
		"m_tcp": "TCP 握手", "m_https": "HTTPS（复用连接）", "m_icmp": "ICMP 回显", "nSamples": "%d 次采样", "from": "出口", "unknownNet": "未知网络",
		"h_provider": "厂商", "h_region": "区域", "h_city": "城市", "h_median": "中位数", "h_min": "最小", "h_p90": "P90", "h_jitter": "抖动", "h_loss": "丢包", "h_tier": "档位", "h_fit": "符合",
		"more": "… 还有 %d 个（--top 0 显示全部）", "best": "各厂商最快区域", "summary": "%d / %d 个区域有响应%s，用时 %s · 节点清单：%s", "noV6": "（另有 %d 个没有 IPv6 地址）",
		"link": "查看和分享：", "fullResult": "完整结果", "topOf": "共 %[2]d 个区域，这里是前 %[1]d 个。", "measuring": "测量中",
		"icmpSkip": "这里用不了 ICMP（需要 root 或 CAP_NET_RAW），只测其他方式", "noMatch": "没有符合这些条件的区域",
		"back": "返回上一步", "exit": "退出", "answered": "%d / %d 个区域有响应%s", "took": "用时 %s · 节点清单：%s",
		"shortFail": "短链服务没有响应，改为显示完整链接",
		"saved":     "已保存：", "saveQ": "同时把结果保存成文件吗？", "saveNo": "不用，看屏幕上的表格就行",
		"updAsk": "发现新版本 v%s（当前 v%s），现在更新吗？", "updNote": "vps2cloud 有新版本 v%s：运行 vps2cloud --update 更新", "updDone": "已更新到 v%s", "updLatest": "vps2cloud v%s 已是最新版本", "updFail": "更新失败：%s", "updPerm": "没有权限替换 %s；请用 sudo 运行，或重新安装：curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "请重新运行 vps2cloud 以使用新版本。",
		"partial": "已中断：以下是已测完的 %d / %d 个区域",
	},
	"zh-TW": {
		"title": "VPS2Cloud %s · 測這台機器到 %d 個雲端區域的延遲", "machine": "本機：%s · Enter 選 [預設] · Ctrl-C 離開",
		"what": "要測什麼？", "all": "全部 %d 個區域", "prov": "依廠商選", "cont": "依大洲選",
		"langItem": "語言", "choice": "選擇", "num": "請輸入 1 到 %d 之間的數字。", "pickProv": "廠商（如 1,3 或 2-5）", "pickCont": "大洲（如 1,2）",
		"pickErr": "請輸入 1 到 %d 之間的一個或多個數字，如 1,3 或 2-5。", "ip": "IP 版本？", "v4": "IPv4", "v6": "IPv6（沒有 IPv6 的區域會略過）", "both": "兩個一起測（一份報告，分 IPv4 和 IPv6 兩部分）",
		"how": "怎麼測？", "tcp": "TCP 交握：只測網路往返", "https": "HTTPS：在已建立的連線上發請求，和應用程式看到的一樣（較費時）",
		"icmp": "ICMP ping（需要 root）", "allModes": "三種都測，並排比較", "samples": "每個區域取樣幾次？", "quick": "快速，5 次", "std": "標準，10 次", "precise": "精確，20 次",
		"again": "下次不用選單，直接執行同樣的測試：", "langPick": "選擇語言", "langSaved": "語言：%s。之後可以在選單裡或用 --lang 修改。",
		"m_tcp": "TCP 交握", "m_https": "HTTPS（重用連線）", "m_icmp": "ICMP 回應", "nSamples": "%d 次取樣", "from": "出口", "unknownNet": "未知網路",
		"h_provider": "廠商", "h_region": "區域", "h_city": "城市", "h_median": "中位數", "h_min": "最小", "h_p90": "P90", "h_jitter": "抖動", "h_loss": "掉包", "h_tier": "等級", "h_fit": "符合",
		"more": "… 還有 %d 個（--top 0 顯示全部）", "best": "各廠商最快區域", "summary": "%d / %d 個區域有回應%s，耗時 %s · 節點清單：%s", "noV6": "（另有 %d 個沒有 IPv6 位址）",
		"link": "檢視與分享：", "fullResult": "完整結果", "topOf": "共 %[2]d 個區域，這裡是前 %[1]d 個。", "measuring": "測量中",
		"icmpSkip": "這裡無法使用 ICMP（需要 root 或 CAP_NET_RAW），只測其他方式", "noMatch": "沒有符合這些條件的區域",
		"back": "返回上一步", "exit": "離開", "answered": "%d / %d 個區域有回應%s", "took": "耗時 %s · 節點清單：%s",
		"shortFail": "短網址服務沒有回應，改為顯示完整連結",
		"saved":     "已儲存：", "saveQ": "同時把結果存成檔案嗎？", "saveNo": "不用，看螢幕上的表格就好",
		"updAsk": "發現新版本 v%s（目前 v%s），現在更新嗎？", "updNote": "vps2cloud 有新版本 v%s：執行 vps2cloud --update 更新", "updDone": "已更新到 v%s", "updLatest": "vps2cloud v%s 已是最新版本", "updFail": "更新失敗：%s", "updPerm": "沒有權限取代 %s；請用 sudo 執行，或重新安裝：curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "請重新執行 vps2cloud 以使用新版本。",
		"partial": "已中斷：以下是已測完的 %d / %d 個區域",
	},
	"ja": {
		"title": "VPS2Cloud %s · このマシンから %d のクラウドリージョンまでの遅延", "machine": "このマシン：%s · Enter で [既定] · Ctrl-C で終了",
		"what": "何を測りますか？", "all": "全 %d リージョン", "prov": "プロバイダーを選ぶ", "cont": "大陸を選ぶ",
		"langItem": "言語", "choice": "選択", "num": "1〜%d の数字を入力してください。", "pickProv": "プロバイダー（例 1,3 や 2-5）", "pickCont": "大陸（例 1,2）",
		"pickErr": "1〜%d の数字を 1 つ以上入力してください（例 1,3 や 2-5）。", "ip": "IP バージョンは？", "v4": "IPv4", "v6": "IPv6（IPv6 のないリージョンは省略）", "both": "両方を一度に（1 つのレポートに IPv4 と IPv6 の節）",
		"how": "測定方法は？", "tcp": "TCP ハンドシェイク：ネットワークの往復だけ", "https": "HTTPS：確立済みの接続でリクエスト、アプリと同じ見え方（時間がかかる）",
		"icmp": "ICMP ping（root が必要）", "allModes": "3 つすべてを並べて", "samples": "リージョンごとの測定回数は？", "quick": "クイック、5 回", "std": "標準、10 回", "precise": "精密、20 回",
		"again": "次回メニューなしで同じテストを実行するには：", "langPick": "言語を選んでください", "langSaved": "言語：%s。メニューか --lang で変更できます。",
		"m_tcp": "TCP ハンドシェイク", "m_https": "HTTPS（接続を再利用）", "m_icmp": "ICMP エコー", "nSamples": "%d 回測定", "from": "出口", "unknownNet": "不明なネットワーク",
		"h_provider": "プロバイダー", "h_region": "リージョン", "h_city": "都市", "h_median": "中央値", "h_min": "最小", "h_p90": "P90", "h_jitter": "ジッター", "h_loss": "損失", "h_tier": "ランク", "h_fit": "適合",
		"more": "… ほか %d 件（--top 0 で全件）", "best": "プロバイダーごとの最速リージョン", "summary": "%d / %d リージョンが応答%s、所要 %s · エンドポイント：%s", "noV6": "（ほか %d 件は IPv6 アドレスなし）",
		"link": "表示と共有：", "fullResult": "全結果", "topOf": "%[2]d リージョン中の上位 %[1]d 件。", "measuring": "測定中",
		"icmpSkip": "ここでは ICMP を使えません（root か CAP_NET_RAW が必要）。ほかの方法で測ります", "noMatch": "条件に合うリージョンがありません",
		"back": "前に戻る", "exit": "終了", "answered": "%d / %d リージョンが応答%s", "took": "所要 %s · エンドポイント：%s",
		"shortFail": "短縮リンクのサービスが応答しないため、完全なリンクを表示します",
		"saved":     "保存しました：", "saveQ": "結果をファイルにも保存しますか？", "saveNo": "いいえ、画面の表で十分",
		"updAsk": "新しいバージョン v%s があります（現在 v%s）。今すぐ更新しますか？", "updNote": "vps2cloud v%s が利用できます：vps2cloud --update で更新", "updDone": "v%s に更新しました", "updLatest": "vps2cloud v%s は最新版です", "updFail": "更新に失敗しました：%s", "updPerm": "%s を置き換える権限がありません。sudo で実行するか再インストールしてください：curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "新しいバージョンを使うには vps2cloud をもう一度実行してください。",
		"partial": "中断しました：測定済みの %d / %d リージョン",
	},
	"ko": {
		"title": "VPS2Cloud %s · 이 머신에서 %d개 클라우드 리전까지의 지연", "machine": "이 머신: %s · Enter는 [기본값] · Ctrl-C는 종료",
		"what": "무엇을 측정할까요?", "all": "전체 %d개 리전", "prov": "프로바이더 선택", "cont": "대륙 선택",
		"langItem": "언어", "choice": "선택", "num": "1부터 %d 사이의 숫자를 입력하세요.", "pickProv": "프로바이더(예: 1,3 또는 2-5)", "pickCont": "대륙(예: 1,2)",
		"pickErr": "1부터 %d 사이의 숫자를 하나 이상 입력하세요(예: 1,3 또는 2-5).", "ip": "IP 버전은?", "v4": "IPv4", "v6": "IPv6(IPv6가 없는 리전은 건너뜀)", "both": "둘 다 한 번에(한 보고서에 IPv4·IPv6 구역)",
		"how": "측정 방법은?", "tcp": "TCP 핸드셰이크: 네트워크 왕복만", "https": "HTTPS: 열린 연결에서 요청, 앱이 보는 그대로(시간이 더 걸림)",
		"icmp": "ICMP ping(root 필요)", "allModes": "세 가지 모두 나란히", "samples": "리전당 측정 횟수는?", "quick": "빠르게, 5회", "std": "표준, 10회", "precise": "정밀, 20회",
		"again": "다음에 메뉴 없이 같은 테스트를 실행하려면:", "langPick": "언어를 선택하세요", "langSaved": "언어: %s. 메뉴나 --lang으로 바꿀 수 있습니다.",
		"m_tcp": "TCP 핸드셰이크", "m_https": "HTTPS(연결 재사용)", "m_icmp": "ICMP 에코", "nSamples": "%d회 측정", "from": "출구", "unknownNet": "알 수 없는 네트워크",
		"h_provider": "프로바이더", "h_region": "리전", "h_city": "도시", "h_median": "중앙값", "h_min": "최소", "h_p90": "P90", "h_jitter": "지터", "h_loss": "손실", "h_tier": "등급", "h_fit": "적합",
		"more": "… %d개 더(--top 0이면 전체)", "best": "프로바이더별 가장 빠른 리전", "summary": "%d / %d개 리전 응답%s, 소요 %s · 엔드포인트: %s", "noV6": "(%d개는 IPv6 주소 없음)",
		"link": "보기 및 공유:", "fullResult": "전체 결과", "topOf": "%[2]d개 리전 중 상위 %[1]d개.", "measuring": "측정 중",
		"icmpSkip": "여기서는 ICMP를 쓸 수 없습니다(root 또는 CAP_NET_RAW 필요). 다른 방법으로 측정합니다", "noMatch": "조건에 맞는 리전이 없습니다",
		"back": "이전으로", "exit": "종료", "answered": "%d / %d개 리전 응답%s", "took": "소요 %s · 엔드포인트: %s",
		"shortFail": "단축 링크 서비스가 응답하지 않아 전체 링크를 표시합니다",
		"saved":     "저장됨:", "saveQ": "결과를 파일로도 저장할까요?", "saveNo": "아니요, 화면의 표면 충분합니다",
		"updAsk": "새 버전 v%s이(가) 있습니다(현재 v%s). 지금 업데이트할까요?", "updNote": "vps2cloud v%s 사용 가능: vps2cloud --update 로 업데이트", "updDone": "v%s(으)로 업데이트했습니다", "updLatest": "vps2cloud v%s 는 최신 버전입니다", "updFail": "업데이트 실패: %s", "updPerm": "%s 을(를) 바꿀 권한이 없습니다. sudo 로 실행하거나 다시 설치하세요: curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "새 버전을 사용하려면 vps2cloud 를 다시 실행하세요.",
		"partial": "중단됨: 지금까지 측정한 %d / %d개 리전",
	},
	"es": {
		"title": "VPS2Cloud %s · latencia desde esta máquina a %d regiones en la nube", "machine": "Esta máquina: %s · Enter deja el [predeterminado] · Ctrl-C sale",
		"what": "¿Qué medir?", "all": "Las %d regiones", "prov": "Elegir proveedores", "cont": "Elegir continentes",
		"langItem": "Idioma", "choice": "Opción", "num": "Escribe un número del 1 al %d.", "pickProv": "Proveedores (p. ej. 1,3 o 2-5)", "pickCont": "Continentes (p. ej. 1,2)",
		"pickErr": "Escribe uno o más números del 1 al %d, como 1,3 o 2-5.", "ip": "¿Versión de IP?", "v4": "IPv4", "v6": "IPv6 (se omiten las regiones sin IPv6)", "both": "Ambas en una sola prueba (un informe con secciones IPv4 e IPv6)",
		"how": "¿Cómo medir?", "tcp": "Handshake TCP: solo el viaje de ida y vuelta de la red", "https": "HTTPS: una petición en una conexión abierta, como la ve una app (tarda más)",
		"icmp": "Ping ICMP (requiere root)", "allModes": "Las tres, lado a lado", "samples": "¿Muestras por región?", "quick": "Rápido, 5", "std": "Estándar, 10", "precise": "Preciso, 20",
		"again": "Para repetir la misma prueba sin el menú:", "langPick": "Elige un idioma", "langSaved": "Idioma: %s. Cámbialo desde el menú o con --lang.",
		"m_tcp": "Handshake TCP", "m_https": "HTTPS (conexión reutilizada)", "m_icmp": "Eco ICMP", "nSamples": "%d muestras", "from": "desde", "unknownNet": "red desconocida",
		"h_provider": "Proveedor", "h_region": "Región", "h_city": "Ciudad", "h_median": "Mediana", "h_min": "Mín", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Pérdida", "h_tier": "Nivel", "h_fit": "Apta",
		"more": "… %d más (--top 0 las muestra todas)", "best": "La mejor región de cada proveedor", "summary": "Respondieron %d de %d regiones%s en %s · endpoints: %s", "noV6": " (otras %d no tienen IPv6)",
		"link": "Ver y compartir:", "fullResult": "Resultado completo", "topOf": "Las %d mejores de %d regiones.", "measuring": "midiendo",
		"icmpSkip": "ICMP no está disponible aquí (requiere root o CAP_NET_RAW); se miden los demás modos", "noMatch": "ninguna región coincide con esos filtros",
		"back": "Atrás", "exit": "Salir", "answered": "Respondieron %d de %d regiones%s", "took": "Duración %s · endpoints: %s",
		"shortFail": "el servicio de enlaces cortos no respondió; se muestra el enlace completo",
		"saved":     "Guardado:", "saveQ": "¿Guardar también el resultado en un archivo?", "saveNo": "No, basta con la tabla en pantalla",
		"updAsk": "Hay una versión nueva, v%s (tienes v%s). ¿Actualizar ahora?", "updNote": "vps2cloud v%s disponible: ejecuta vps2cloud --update", "updDone": "Actualizado a v%s", "updLatest": "vps2cloud v%s es la última versión", "updFail": "no se pudo actualizar: %s", "updPerm": "sin permiso para reemplazar %s; usa sudo o reinstala: curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "Vuelve a ejecutar vps2cloud para usar la versión nueva.",
		"partial": "interrumpido: las %d de %d regiones medidas hasta ahora",
	},
	"fr": {
		"title": "VPS2Cloud %s · latence de cette machine vers %d régions cloud", "machine": "Cette machine : %s · Entrée garde la [valeur par défaut] · Ctrl-C quitte",
		"what": "Que mesurer ?", "all": "Les %d régions", "prov": "Choisir des fournisseurs", "cont": "Choisir des continents",
		"langItem": "Langue", "choice": "Choix", "num": "Entrez un nombre de 1 à %d.", "pickProv": "Fournisseurs (ex. 1,3 ou 2-5)", "pickCont": "Continents (ex. 1,2)",
		"pickErr": "Entrez un ou plusieurs nombres de 1 à %d, comme 1,3 ou 2-5.", "ip": "Version d’IP ?", "v4": "IPv4", "v6": "IPv6 (les régions sans IPv6 sont ignorées)", "both": "Les deux en un seul test (un rapport, sections IPv4 et IPv6)",
		"how": "Comment mesurer ?", "tcp": "Handshake TCP : l’aller-retour réseau seul", "https": "HTTPS : une requête sur une connexion ouverte, comme la voit une app (plus long)",
		"icmp": "Ping ICMP (nécessite root)", "allModes": "Les trois côte à côte", "samples": "Mesures par région ?", "quick": "Rapide, 5", "std": "Standard, 10", "precise": "Précis, 20",
		"again": "Pour relancer le même test sans le menu :", "langPick": "Choisissez une langue", "langSaved": "Langue : %s. Modifiable depuis le menu ou avec --lang.",
		"m_tcp": "Handshake TCP", "m_https": "HTTPS (connexion réutilisée)", "m_icmp": "Écho ICMP", "nSamples": "%d mesures", "from": "depuis", "unknownNet": "réseau inconnu",
		"h_provider": "Fournisseur", "h_region": "Région", "h_city": "Ville", "h_median": "Médiane", "h_min": "Min", "h_p90": "P90", "h_jitter": "Gigue", "h_loss": "Perte", "h_tier": "Niveau", "h_fit": "OK",
		"more": "… %d de plus (--top 0 affiche tout)", "best": "Meilleure région par fournisseur", "summary": "%d régions sur %d ont répondu%s en %s · endpoints : %s", "noV6": " (%d autres n’ont pas d’IPv6)",
		"link": "Voir et partager :", "fullResult": "Résultat complet", "topOf": "Les %d meilleures sur %d régions.", "measuring": "mesure",
		"icmpSkip": "ICMP n’est pas disponible ici (nécessite root ou CAP_NET_RAW) ; mesure des autres modes", "noMatch": "aucune région ne correspond à ces filtres",
		"back": "Retour", "exit": "Quitter", "answered": "%d régions sur %d ont répondu%s", "took": "Durée %s · endpoints : %s",
		"shortFail": "le service de liens courts n’a pas répondu ; lien complet affiché",
		"saved":     "Enregistré :", "saveQ": "Enregistrer aussi le résultat dans un fichier ?", "saveNo": "Non, le tableau à l’écran suffit",
		"updAsk": "La version v%s est disponible (vous avez v%s). Mettre à jour maintenant ?", "updNote": "vps2cloud v%s est disponible : lancez vps2cloud --update", "updDone": "Mis à jour en v%s", "updLatest": "vps2cloud v%s est la dernière version", "updFail": "échec de la mise à jour : %s", "updPerm": "pas le droit de remplacer %s ; utilisez sudo ou réinstallez : curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "Relancez vps2cloud pour utiliser la nouvelle version.",
		"partial": "interrompu : les %d régions mesurées sur %d",
	},
	"de": {
		"title": "VPS2Cloud %s · Latenz von diesem Rechner zu %d Cloud-Regionen", "machine": "Dieser Rechner: %s · Enter übernimmt den [Standard] · Strg-C beendet",
		"what": "Was messen?", "all": "Alle %d Regionen", "prov": "Anbieter wählen", "cont": "Kontinente wählen",
		"langItem": "Sprache", "choice": "Auswahl", "num": "Gib eine Zahl von 1 bis %d ein.", "pickProv": "Anbieter (z. B. 1,3 oder 2-5)", "pickCont": "Kontinente (z. B. 1,2)",
		"pickErr": "Gib eine oder mehrere Zahlen von 1 bis %d ein, etwa 1,3 oder 2-5.", "ip": "IP-Version?", "v4": "IPv4", "v6": "IPv6 (Regionen ohne IPv6 werden übersprungen)", "both": "Beide in einem Lauf (ein Bericht mit IPv4- und IPv6-Teil)",
		"how": "Wie messen?", "tcp": "TCP-Handshake: nur die Netzwerk-Laufzeit", "https": "HTTPS: eine Anfrage über eine offene Verbindung, wie eine App sie sieht (dauert länger)",
		"icmp": "ICMP-Ping (braucht root)", "allModes": "Alle drei nebeneinander", "samples": "Messungen pro Region?", "quick": "Schnell, 5", "std": "Standard, 10", "precise": "Genau, 20",
		"again": "Denselben Test ohne Menü wiederholen:", "langPick": "Sprache wählen", "langSaved": "Sprache: %s. Änderbar im Menü oder mit --lang.",
		"m_tcp": "TCP-Handshake", "m_https": "HTTPS (wiederverwendete Verbindung)", "m_icmp": "ICMP-Echo", "nSamples": "%d Messungen", "from": "von", "unknownNet": "unbekanntes Netz",
		"h_provider": "Anbieter", "h_region": "Region", "h_city": "Stadt", "h_median": "Median", "h_min": "Min", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Verlust", "h_tier": "Stufe", "h_fit": "Passt",
		"more": "… %d weitere (--top 0 zeigt alle)", "best": "Beste Region je Anbieter", "summary": "%d von %d Regionen haben geantwortet%s in %s · Endpunkte: %s", "noV6": " (%d weitere haben kein IPv6)",
		"link": "Ansehen und teilen:", "fullResult": "Vollständiges Ergebnis", "topOf": "Die besten %d von %d Regionen.", "measuring": "Messung",
		"icmpSkip": "ICMP ist hier nicht verfügbar (braucht root oder CAP_NET_RAW); die anderen Methoden werden gemessen", "noMatch": "keine Region passt zu diesen Filtern",
		"back": "Zurück", "exit": "Beenden", "answered": "%d von %d Regionen haben geantwortet%s", "took": "Dauer %s · Endpunkte: %s",
		"shortFail": "der Kurzlink-Dienst hat nicht geantwortet; der vollständige Link wird angezeigt",
		"saved":     "Gespeichert:", "saveQ": "Ergebnis zusätzlich als Datei speichern?", "saveNo": "Nein, die Tabelle am Bildschirm reicht",
		"updAsk": "Version v%s ist da (du hast v%s). Jetzt aktualisieren?", "updNote": "vps2cloud v%s ist verfügbar: vps2cloud --update ausführen", "updDone": "Auf v%s aktualisiert", "updLatest": "vps2cloud v%s ist die neueste Version", "updFail": "Aktualisierung fehlgeschlagen: %s", "updPerm": "keine Berechtigung, %s zu ersetzen; mit sudo ausführen oder neu installieren: curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "Starte vps2cloud erneut, um die neue Version zu nutzen.",
		"partial": "abgebrochen: die bisher gemessenen %d von %d Regionen",
	},
	"pt": {
		"title": "VPS2Cloud %s · latência desta máquina para %d regiões de nuvem", "machine": "Esta máquina: %s · Enter mantém o [padrão] · Ctrl-C sai",
		"what": "O que medir?", "all": "Todas as %d regiões", "prov": "Escolher provedores", "cont": "Escolher continentes",
		"langItem": "Idioma", "choice": "Opção", "num": "Digite um número de 1 a %d.", "pickProv": "Provedores (ex. 1,3 ou 2-5)", "pickCont": "Continentes (ex. 1,2)",
		"pickErr": "Digite um ou mais números de 1 a %d, como 1,3 ou 2-5.", "ip": "Versão do IP?", "v4": "IPv4", "v6": "IPv6 (regiões sem IPv6 são puladas)", "both": "Ambas num só teste (um relatório com seções IPv4 e IPv6)",
		"how": "Como medir?", "tcp": "Handshake TCP: só a ida e volta da rede", "https": "HTTPS: uma requisição numa conexão aberta, como um app vê (demora mais)",
		"icmp": "Ping ICMP (requer root)", "allModes": "As três lado a lado", "samples": "Amostras por região?", "quick": "Rápido, 5", "std": "Padrão, 10", "precise": "Preciso, 20",
		"again": "Para repetir o mesmo teste sem o menu:", "langPick": "Escolha um idioma", "langSaved": "Idioma: %s. Altere pelo menu ou com --lang.",
		"m_tcp": "Handshake TCP", "m_https": "HTTPS (conexão reutilizada)", "m_icmp": "Eco ICMP", "nSamples": "%d amostras", "from": "de", "unknownNet": "rede desconhecida",
		"h_provider": "Provedor", "h_region": "Região", "h_city": "Cidade", "h_median": "Mediana", "h_min": "Mín", "h_p90": "P90", "h_jitter": "Jitter", "h_loss": "Perda", "h_tier": "Nível", "h_fit": "Serve",
		"more": "… mais %d (--top 0 mostra todas)", "best": "Melhor região de cada provedor", "summary": "%d de %d regiões responderam%s em %s · endpoints: %s", "noV6": " (outras %d não têm IPv6)",
		"link": "Ver e compartilhar:", "fullResult": "Resultado completo", "topOf": "As %d melhores de %d regiões.", "measuring": "medindo",
		"icmpSkip": "ICMP não está disponível aqui (requer root ou CAP_NET_RAW); medindo os outros modos", "noMatch": "nenhuma região corresponde a esses filtros",
		"back": "Voltar", "exit": "Sair", "answered": "%d de %d regiões responderam%s", "took": "Duração %s · endpoints: %s",
		"shortFail": "o serviço de links curtos não respondeu; mostrando o link completo",
		"saved":     "Salvo:", "saveQ": "Salvar também o resultado num arquivo?", "saveNo": "Não, a tabela na tela basta",
		"updAsk": "Há uma versão nova, v%s (você tem v%s). Atualizar agora?", "updNote": "vps2cloud v%s disponível: execute vps2cloud --update", "updDone": "Atualizado para v%s", "updLatest": "vps2cloud v%s é a versão mais recente", "updFail": "falha ao atualizar: %s", "updPerm": "sem permissão para substituir %s; use sudo ou reinstale: curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "Execute vps2cloud de novo para usar a versão nova.",
		"partial": "interrompido: as %d de %d regiões medidas até agora",
	},
	"ru": {
		"title": "VPS2Cloud %s · задержка от этой машины до %d облачных регионов", "machine": "Эта машина: %s · Enter оставляет [по умолчанию] · Ctrl-C — выход",
		"what": "Что измерить?", "all": "Все %d регионов", "prov": "Выбрать провайдеров", "cont": "Выбрать континенты",
		"langItem": "Язык", "choice": "Выбор", "num": "Введите число от 1 до %d.", "pickProv": "Провайдеры (напр. 1,3 или 2-5)", "pickCont": "Континенты (напр. 1,2)",
		"pickErr": "Введите одно или несколько чисел от 1 до %d, например 1,3 или 2-5.", "ip": "Версия IP?", "v4": "IPv4", "v6": "IPv6 (регионы без IPv6 пропускаются)", "both": "Обе за один прогон (один отчёт с разделами IPv4 и IPv6)",
		"how": "Как измерять?", "tcp": "TCP-рукопожатие: только сетевой путь туда и обратно", "https": "HTTPS: запрос по открытому соединению, как его видит приложение (дольше)",
		"icmp": "ICMP ping (нужен root)", "allModes": "Все три рядом", "samples": "Замеров на регион?", "quick": "Быстро, 5", "std": "Стандартно, 10", "precise": "Точно, 20",
		"again": "Чтобы повторить тот же тест без меню:", "langPick": "Выберите язык", "langSaved": "Язык: %s. Его можно сменить в меню или через --lang.",
		"m_tcp": "TCP-рукопожатие", "m_https": "HTTPS (повторное соединение)", "m_icmp": "ICMP-эхо", "nSamples": "%d замеров", "from": "откуда", "unknownNet": "неизвестная сеть",
		"h_provider": "Провайдер", "h_region": "Регион", "h_city": "Город", "h_median": "Медиана", "h_min": "Мин", "h_p90": "P90", "h_jitter": "Джиттер", "h_loss": "Потери", "h_tier": "Уровень", "h_fit": "Подходит",
		"more": "… ещё %d (--top 0 покажет все)", "best": "Лучший регион каждого провайдера", "summary": "Ответили %d из %d регионов%s за %s · эндпоинты: %s", "noV6": " (ещё у %d нет IPv6)",
		"link": "Посмотреть и поделиться:", "fullResult": "Полный результат", "topOf": "Лучшие %d из %d регионов.", "measuring": "измерение",
		"icmpSkip": "ICMP здесь недоступен (нужен root или CAP_NET_RAW); измеряем остальными способами", "noMatch": "ни один регион не подходит под эти фильтры",
		"back": "Назад", "exit": "Выход", "answered": "Ответили %d из %d регионов%s", "took": "Заняло %s · эндпоинты: %s",
		"shortFail": "сервис коротких ссылок не ответил; показана полная ссылка",
		"saved":     "Сохранено:", "saveQ": "Сохранить результат ещё и в файл?", "saveNo": "Нет, достаточно таблицы на экране",
		"updAsk": "Вышла версия v%s (у вас v%s). Обновить сейчас?", "updNote": "Доступна vps2cloud v%s: выполните vps2cloud --update", "updDone": "Обновлено до v%s", "updLatest": "vps2cloud v%s — последняя версия", "updFail": "не удалось обновить: %s", "updPerm": "нет прав заменить %s; запустите через sudo или переустановите: curl -fsSL https://v2c.1ms.dog/i | sh", "updRestart": "Запустите vps2cloud снова, чтобы использовать новую версию.",
		"partial": "прервано: измерено %d из %d регионов",
	},
}
