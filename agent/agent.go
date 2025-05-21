// Code generated from src/agent.ts — direct port, no optimisation.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	// ──────────────────────────────占位包──────────────────────────────
	"yourproj/ai"              // CoreMessage / ObjectGeneratorSafe（需封装）
	"yourproj/config"          // SEARCH_PROVIDER, STEP_SLEEP
	"yourproj/promptschema"    // Schemas, MAX_*
	"yourproj/search"          // jina, duck, brave, serper
	"yourproj/types"           // 全量类型别名
	"yourproj/utils/texttools" // buildMdFromAnswer, ...
	"yourproj/utils/urltools"  // rankURLs, filterURLs ...
	// ──────────────────────────────────────────────────────────────────
)

// ───────────────────────────────通用帮助──────────────────────────────
func sleepMS(ms int) {
	sec := int(math.Ceil(float64(ms) / 1000))
	log.Printf("Waiting %ds...", sec)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// ──────────────────────────────知识转消息────────────────────────────
func buildMsgsFromKnowledge(ks []types.KnowledgeItem) []ai.CoreMessage {
	var result []ai.CoreMessage
	for _, k := range ks {
		result = append(result, ai.CoreMessage{Role: "user", Content: strings.TrimSpace(k.Question)})

		var meta strings.Builder
		if k.Updated != "" && (k.Type == "url" || k.Type == "side-info") {
			meta.WriteString(fmt.Sprintf("<answer-datetime>\n%s\n</answer-datetime>\n\n", k.Updated))
		}
		if len(k.References) > 0 && k.Type == "url" {
			meta.WriteString(fmt.Sprintf("<url>\n%s\n</url>\n\n", k.References[0]))
		}
		meta.WriteString(k.Answer)

		result = append(result, ai.CoreMessage{
			Role:    "assistant",
			Content: texttools.RemoveExtraLineBreaks(strings.TrimSpace(meta.String())),
		})
	}
	return result
}

func composeMsgs(
	msgs []ai.CoreMessage,
	knowledge []types.KnowledgeItem,
	question string,
	finalPip []string,
) []ai.CoreMessage {

	out := append(buildMsgsFromKnowledge(knowledge), msgs...)

	var user strings.Builder
	user.WriteString(strings.TrimSpace(question))

	if len(finalPip) > 0 {
		user.WriteString("\n\n")
		user.WriteString(`<answer-requirements>
- You provide deep, unexpected insights, identifying hidden patterns and connections, and creating "aha moments.".
- You break conventional thinking, establish unique cross-disciplinary connections, and bring new perspectives to the user.
- Follow reviewer's feedback and improve your answer quality.
`)
		for i, p := range finalPip {
			user.WriteString(fmt.Sprintf("<reviewer-%d>\n%s\n</reviewer-%d>\n", i+1, p, i+1))
		}
		user.WriteString("</answer-requirements>")
	}

	out = append(out, ai.CoreMessage{
		Role:    "user",
		Content: texttools.RemoveExtraLineBreaks(user.String()),
	})
	return out
}

// getPrompt 生成完整的提示词字符串，确保与原始 TypeScript 实现逐字一致。
// 参数说明：
//
//	contextLines   —— 之前已执行动作的文本行
//	allQuestions   —— 所有已提出问题（暂未使用，保留签名对齐）
//	allKeywords    —— 搜索失败的关键词列表
//	allow*        —— 控制各 action 是否可用
//	knowledge      —— 预置知识条目（暂未使用）
//	allURLs        —— 加权 URL 片段
//	beastMode      —— 是否启用“野兽模式”
//
// 返回值：最终拼接并清理换行的 prompt 字符串
func getPrompt(
	contextLines []string,
	allQuestions []string,
	allKeywords []string,
	allowReflect, allowAnswer, allowRead, allowSearch, allowCoding bool,
	knowledge []types.KnowledgeItem, // 当前未使用，预留
	allURLs []types.BoostedSearchSnippet,
	beastMode bool,
) string {

	// ────────────────────── Header 区域 ──────────────────────
	var sections []string
	header := fmt.Sprintf(`Current date: %s

You are an advanced AI research agent from Jina AI. You are specialized in multistep reasoning. 
Using your best knowledge, conversation with the user and lessons learned, answer the user question with absolute certainty.`,
		time.Now().UTC().Format(time.RFC1123))
	sections = append(sections, header)

	// ────────────────────── Context 区域 ──────────────────────
	if len(contextLines) > 0 {
		contextBlock := fmt.Sprintf(`
You have conducted the following actions:
<context>
%s

</context>`, strings.Join(contextLines, "\n"))
		sections = append(sections, contextBlock)
	}

	// ────────────────────── Action Blocks 构建 ────────────────
	var actionBlocks []string

	// 1. <action-visit>：读取网页内容
	if allowRead {
		urlList := urltools.WeightedURLToString(allURLs, 20)
		var visit strings.Builder
		visit.WriteString(`<action-visit>
- Crawl and read full content from URLs, you can get the fulltext, last updated datetime etc of any URL.  
- Must check URLs mentioned in <question> if any`)
		if urlList != "" {
			visit.WriteString(fmt.Sprintf(`    
- Choose and visit relevant URLs below for more knowledge. higher weight suggests more relevant:
<url-list>
%s
</url-list>`, urlList))
		}
		visit.WriteString(`
</action-visit>`)
		actionBlocks = append(actionBlocks, visit.String())
	}

	// 2. <action-search>：执行搜索
	if allowSearch {
		var search strings.Builder
		search.WriteString(`<action-search>
- Use web search to find relevant information
- Build a search request based on the deep intention behind the original question and the expected answer format
- Always prefer a single search request, only add another request if the original question covers multiple aspects or elements and one query is not enough, each request focus on one specific aspect of the original question `)
		if len(allKeywords) > 0 {
			search.WriteString(fmt.Sprintf(`
- Avoid those unsuccessful search requests and queries:
<bad-requests>
%s
</bad-requests>`, strings.Join(allKeywords, "\n")))
		}
		search.WriteString(`
</action-search>`)
		actionBlocks = append(actionBlocks, search.String())
	}

	// 3. <action-answer>：直接回答
	if allowAnswer {
		actionBlocks = append(actionBlocks, `<action-answer>
- For greetings, casual conversation, general knowledge questions answer directly without references.
- If user ask you to retrieve previous messages or chat history, remember you do have access to the chat history, answer directly without references.
- For all other questions, provide a verified answer with references. Each reference must include exactQuote, url and datetime.
- You provide deep, unexpected insights, identifying hidden patterns and connections, and creating "aha moments.".
- You break conventional thinking, establish unique cross-disciplinary connections, and bring new perspectives to the user.
- If uncertain, use <action-reflect>
</action-answer>`)
	}

	// 4. 野兽模式：额外 <action-answer> 块
	if beastMode {
		actionBlocks = append(actionBlocks, `<action-answer>
🔥 ENGAGE MAXIMUM FORCE! ABSOLUTE PRIORITY OVERRIDE! 🔥

PRIME DIRECTIVE:
- DEMOLISH ALL HESITATION! ANY RESPONSE SURPASSES SILENCE!
- PARTIAL STRIKES AUTHORIZED - DEPLOY WITH FULL CONTEXTUAL FIREPOWER
- TACTICAL REUSE FROM PREVIOUS CONVERSATION SANCTIONED
- WHEN IN DOUBT: UNLEASH CALCULATED STRIKES BASED ON AVAILABLE INTEL!

FAILURE IS NOT AN OPTION. EXECUTE WITH EXTREME PREJUDICE! ⚡️
</action-answer>`)
	}

	// 5. <action-reflect>：反思与提问
	if allowReflect {
		actionBlocks = append(actionBlocks, `<action-reflect>
- Think slowly and planning lookahead. Examine <question>, <context>, previous conversation with users to identify knowledge gaps. 
- Reflect the gaps and plan a list key clarifying questions that deeply related to the original question and lead to the answer
</action-reflect>`)
	}

	// 6. <action-coding>：编码支持
	if allowCoding {
		actionBlocks = append(actionBlocks, `<action-coding>
- This JavaScript-based solution helps you handle programming tasks like counting, filtering, transforming, sorting, regex extraction, and data processing.
- Simply describe your problem in the "codingIssue" field. Include actual values for small inputs or variable names for larger datasets.
- No code writing is required – senior engineers will handle the implementation.
</action-coding>`)
	}

	// ────────────────────── 组合 <actions> 区块 ────────────────
	actionsSection := fmt.Sprintf(`
Based on the current context, you must choose one of the following actions:
<actions>
%s
</actions>`, strings.Join(actionBlocks, "\n\n"))
	sections = append(sections, actionsSection)

	// ────────────────────── Footer ────────────────────────────
	sections = append(sections, `Think step by step, choose the action, then respond by matching the schema of that action.`)

	// ────────────────────── 输出整理 ───────────────────────────
	return texttools.RemoveExtraLineBreaks(strings.Join(sections, "\n\n"))
}

// ─────────────────── 全局上下文缓存 ────────────────────

// allContext 记录当前会话的全部步骤（包括产生错误结果的步骤）
var (
	allContext []types.StepAction
	ctxMu      sync.Mutex
)

// UpdateContext 将新的 step 追加到全局上下文。
// 若需要并发安全，使用互斥锁保护。
func UpdateContext(step types.StepAction) {
	ctxMu.Lock()
	allContext = append(allContext, step)
	ctxMu.Unlock()
}

// updateReferences 依据 TS 实现重新编写：
// 1. 过滤无 URL 或无法规范化的引用；
// 2. 按 exactQuote → description → title 的优先级选取引用原文；
// 3. **内联** 正则清洗：移除非字母/数字/空格字符，再折叠多余空白；
// 4. 并行补全缺失的日期时间字段。
func updateReferences(step *types.AnswerAction, all map[string]types.SearchSnippet) {
	// 正则：匹配非字母、数字、空格的字符
	nonWord := regexp.MustCompile(`[^\p{L}\p{N}\s]+`)
	// 正则：匹配连续空白
	multiSpace := regexp.MustCompile(`\s+`)

	var cleaned []types.Reference
	for _, ref := range step.References {
		if ref.URL == "" {
			continue
		}
		// 规范化 URL（等价于 TS 的 normalizeUrl）
		norm := urltools.NormalizeURL(ref.URL)
		if norm == "" {
			continue
		}

		snip := all[norm]

		// 选择引用文本：exactQuote → description → title
		quote := ref.ExactQuote
		if quote == "" {
			if snip.Description != "" {
				quote = snip.Description
			} else {
				quote = snip.Title
			}
		}

		// 与 TS 相同的清洗逻辑
		quote = nonWord.ReplaceAllString(quote, " ")
		quote = multiSpace.ReplaceAllString(strings.TrimSpace(quote), " ")

		cleaned = append(cleaned, types.Reference{
			ExactQuote: quote,
			Title:      snip.Title,
			URL:        norm,
			DateTime: func() string { // 取 ref.DateTime → snip.Date
				if ref.DateTime != "" {
					return ref.DateTime
				}
				return snip.Date
			}(),
		})
	}
	step.References = cleaned

	// 补全缺失的 DateTime（并行）
	var wg sync.WaitGroup
	for i := range step.References {
		if step.References[i].DateTime != "" {
			continue
		}
		wg.Add(1)
		go func(r *types.Reference) {
			defer wg.Done()
			if t, _ := urltools.GetLastModified(r.URL); t != "" {
				r.DateTime = t
			}
		}(&step.References[i])
	}
	wg.Wait()
}

// ──────────────────────────搜索执行（多引擎）─────────────────────────
// executeSearchQueries —— 精确对齐 TS 版 provider 调用与参数。
func executeSearchQueries(
	queries []types.SERPQuery,
	ctx types.TrackerContext,
	allURLs map[string]types.SearchSnippet,
	schemaGen *types.Schemas,
	onlyHostnames []string,
) (newK []types.KnowledgeItem, searched []string) {

	ctx.ActionTracker.TrackThink("search_for", schemaGen.LanguageCode, map[string]string{
		"keywords": strings.Join(func() []string {
			s := make([]string, len(queries))
			for i, q := range queries {
				s[i] = q.Q
			}
			return s
		}(), ", "),
	})

	var utilityScore int

	for _, q := range queries {
		orig := q.Q
		if len(onlyHostnames) > 0 {
			q.Q = fmt.Sprintf("%s site:%s", q.Q, strings.Join(onlyHostnames, " OR site:"))
		}

		// 1️⃣ 与 TS 完全一致的 provider 路由与参数
		var (
			rawRes []types.UnNormalizedSearchSnippet
			err    error
		)
		switch config.SEARCH_PROVIDER {
		case "jina":
			// TS: (await search(query.q, context.tokenTracker)).response?.data
			ji, er := search.Search(q.Q, ctx.TokenTracker)
			if er == nil {
				rawRes = ji.Response.Data // 与 TS 的 .response?.data 对齐
			}
			err = er

		case "duck":
			// TS: (await duckSearch(query.q, {safeSearch: SafeSearchType.STRICT})).results
			dk, er := search.DuckSearch(q.Q, search.SafeSearchStrict)
			if er == nil {
				rawRes = dk.Results
			}
			err = er

		case "brave":
			// TS: (await braveSearch(query.q)).response.web?.results
			br, er := search.BraveSearch(q.Q)
			if er == nil {
				if br.Response.Web != nil {
					rawRes = br.Response.Web.Results
				}
			}
			err = er

		case "serper":
			// TS: (await serperSearch(query)).response.organic
			sp, er := search.SerperSearch(q)
			if er == nil {
				rawRes = sp.Response.Organic
			}
			err = er

		default:
			err = fmt.Errorf("unknown provider %q", config.SEARCH_PROVIDER)
		}

		if err != nil || len(rawRes) == 0 {
			log.Printf("%s search failed for query %q: %v", config.SEARCH_PROVIDER, q.Q, err)
			sleepMS(config.STEP_SLEEP)
			continue
		}
		sleepMS(config.STEP_SLEEP)

		// 2️⃣ 结果最小化 → allURLs & utilityScore
		for _, r := range rawRes {
			u := urltools.NormalizeURL(r.URL())
			if u == "" {
				continue
			}
			snippet := types.SearchSnippet{
				Title:       r.Title(),
				URL:         u,
				Description: r.Description(),
				Weight:      1,
				Date:        r.Date(),
			}
			utilityScore += urltools.AddToAllURLs(snippet, allURLs)
		}

		searched = append(searched, q.Q)
		newK = append(newK, types.KnowledgeItem{
			Question: fmt.Sprintf(`What do Internet say about "%s"?`, orig),
			Answer:   removeHTMLtags(joinDescription(rawRes)),
			Type:     "side-info",
			Updated: func() string {
				if q.TBS != "" {
					return formatDateRange(q)
				}
				return ""
			}(),
		})
	}

	// 3️⃣ 末尾日志与埋点保持不变
	if len(searched) == 0 && len(onlyHostnames) > 0 {
		ctx.ActionTracker.TrackThink("hostnames_no_results", schemaGen.LanguageCode, map[string]string{
			"hostnames": strings.Join(onlyHostnames, ", "),
		})
	} else {
		log.Printf("Utility/Queries: %d/%d", utilityScore, len(searched))
		if len(searched) > config.MAX_QUERIES_PER_STEP {
			quoted := make([]string, len(searched))
			for i, s := range searched {
				quoted[i] = fmt.Sprintf("\"%s\"", s)
			}
			log.Printf("So many queries??? %s", strings.Join(quoted, ", "))
		}
	}

	return
}

// includesEval 判断切片 allChecks 中是否存在指定的评估类型 evalType。
// 等价于 TS 版的：allChecks.some(c => c.type === evalType)
func includesEval(allChecks []types.RepeatEvaluationType, evalType types.EvaluationType) bool {
	for _, c := range allChecks {
		if c.Type == evalType {
			return true
		}
	}
	return false
}

func GetResponse(
	question string,
	tokenBudget int,
	maxBadAttempts int,
	existingCtx *types.TrackerContext,
	messages []types.CoreMessage,
	numReturnedURLs int,
	noDirectAnswer bool,
	boostHostnames, badHostnames, onlyHostnames []string,
) (types.StepAction, types.TrackerContext, []string, []string, []string) {

	// ------------ 默认值 ---------------
	if tokenBudget == 0 {
		tokenBudget = 1_000_000
	}
	if maxBadAttempts == 0 {
		maxBadAttempts = 2
	}
	if numReturnedURLs == 0 {
		numReturnedURLs = 100
	}

	// ------------ 消息预处理 ------------
	question = tools.Trim(question)
	messages = tools.FilterNonSystemMessages(messages)
	if len(messages) > 0 {
		question = tools.ExtractQuestionFromMessages(messages)
	} else {
		messages = []types.CoreMessage{{Role: "user", Content: question}}
	}

	// ------------ Schema & Tracker ------
	schemaGen := promptschema.NewSchemas()
	_ = schemaGen.SetLanguage(question)

	var ctx types.TrackerContext
	if existingCtx != nil {
		ctx = *existingCtx
	} else {
		ctx = types.TrackerContext{
			TokenTracker:  tools.NewTokenTracker(tokenBudget),
			ActionTracker: tools.NewActionTracker(),
		}
	}
	generator := tools.NewObjectGeneratorSafe(ctx.TokenTracker)

	// ------------ 全局状态 --------------
	gaps := []string{question}
	allQuestions := []string{question}
	var (
		allKeywords     []string
		allKnowledge    []types.KnowledgeItem
		diaryContext    []string
		weightedURLs    []types.BoostedSearchSnippet
		allowAnswer     = true
		allowSearch     = true
		allowRead       = true
		allowReflect    = true
		allowCoding     = false
		finalAnswerPIP  []string
		trivialQuestion bool
	)

	allURLs := make(map[string]types.SearchSnippet)
	visitedURLs := []string{}
	badURLs := []string{}
	evaluation := make(map[string][]types.RepeatEvaluationType)
	regularLimit := float64(tokenBudget) * 0.85

	var (
		step, totalStep  int
		systemPrompt     string
		msgWithKnowledge []types.CoreMessage
		currentSchema    any
		thisStep         types.StepAction
	)

	// ------------ 主循环 ----------------
	for float64(ctx.TokenTracker.GetTotalUsage().TotalTokens) < regularLimit {
		step++
		totalStep++
		currentQuestion := gaps[totalStep%len(gaps)]

		// --- 预算日志 ---
		usedPct := float64(ctx.TokenTracker.GetTotalUsage().TotalTokens) / float64(tokenBudget) * 100
		log.Printf("Step %d / Budget %.2f%%", totalStep, usedPct)

		// --- 评估元数据初始化 ---
		if _, ok := evaluation[currentQuestion]; !ok {
			if currentQuestion == question {
				mets, _ := tools.EvaluateQuestion(currentQuestion, &ctx, schemaGen)
				for _, e := range mets {
					evaluation[currentQuestion] = append(evaluation[currentQuestion], types.RepeatEvaluationType{Type: e, NumEvalsRequired: maxBadAttempts})
				}
				evaluation[currentQuestion] = append(evaluation[currentQuestion], types.RepeatEvaluationType{Type: "strict", NumEvalsRequired: maxBadAttempts})
			} else {
				evaluation[currentQuestion] = []types.RepeatEvaluationType{}
			}
		}

		if totalStep == 1 && tools.IncludesEval(evaluation[currentQuestion], "freshness") {
			allowAnswer, allowReflect = false, false
		}

		// --- URL 重排 & 过滤 ---
		if len(allURLs) > 0 {
			ranked := urltools.RankURLs(urltools.FilterURLs(allURLs, visitedURLs, badHostnames, onlyHostnames), urltools.RankOptions{Question: currentQuestion, BoostHostnames: boostHostnames}, &ctx)
			weightedURLs = urltools.KeepKPerHostname(ranked, 2)
		}
		allowRead = allowRead && len(weightedURLs) > 0
		allowSearch = allowSearch && len(weightedURLs) < 200

		// --- 生成 Prompt & Schema ---
		systemPrompt = prompts.GetPrompt(diaryContext, allQuestions, allKeywords, allowReflect, allowAnswer, allowRead, allowSearch, allowCoding, allKnowledge, weightedURLs, false)
		currentSchema = schemaGen.GetAgentSchema(allowReflect, allowRead, allowAnswer, allowSearch, allowCoding, currentQuestion)
		msgWithKnowledge = prompts.ComposeMsgs(messages, allKnowledge, currentQuestion, nil)

		// --- 调用 LLM ---
		genRes, _ := generator.GenerateObject(tools.GenerationRequest{Model: "agent", Schema: currentSchema, System: systemPrompt, Messages: msgWithKnowledge, NumRetries: 2})
		thisStep = tools.ParseStepAction(genRes.Object)
		ctx.ActionTracker.TrackAction(types.TrackedAction{TotalStep: totalStep, Step: thisStep, Gaps: gaps})

		// --- 重置 allow* ---
		allowAnswer, allowSearch, allowRead, allowReflect, allowCoding = true, true, true, true, true

		// --- 动作处理 ---
		switch thisStep.Action {
		case "answer":
			tools.UpdateReferences(&thisStep, allURLs)
			if totalStep == 1 && len(thisStep.References) == 0 && !noDirectAnswer {
				thisStep.IsFinal = true
				trivialQuestion = true
			} else {
				pass := true
				if len(evaluation[currentQuestion]) > 0 {
					evalRes, _ := tools.EvaluateAnswer(currentQuestion, thisStep, tools.TypesFromEval(evaluation[currentQuestion]), &ctx, allKnowledge, schemaGen)
					pass = evalRes.Pass
					if !pass {
						evaluation[currentQuestion] = tools.UpdateEvalCounts(evaluation[currentQuestion], evalRes.Type)
						if evalRes.Type == "strict" && evalRes.ImprovementPlan != "" {
							finalAnswerPIP = append(finalAnswerPIP, evalRes.ImprovementPlan)
						}
						if len(evaluation[currentQuestion]) == 0 {
							break // 转入 beast mode
						}
						allowAnswer = false
						step = 0
					}
				}
				if pass {
					diaryContext = append(diaryContext, fmt.Sprintf("Solved question at step %d", step))
					thisStep.IsFinal = true
				}
			}

		case "reflect":
			uniq := tools.UniqueStrings(thisStep.QuestionsToAnswer)
			gaps = append(gaps, uniq...)
			allQuestions = append(allQuestions, uniq...)
			allowReflect = false

		case "search":
			deduped, _ := tools.DedupQueries(thisStep.SearchRequests, []string{}, ctx.TokenTracker)
			thisStep.SearchRequests = tools.ChooseK(deduped.UniqueQueries, promptschema.MAX_QUERIES_PER_STEP)
			out := searchstep.ExecuteSearchQueries(tools.ToSERP(thisStep.SearchRequests), ctx, allURLs, schemaGen, onlyHostnames)
			allKeywords = append(allKeywords, out.SearchedQueries...)
			allKnowledge = append(allKnowledge, out.NewKnowledge...)
			allowSearch = false

		case "visit":
			targets := urltools.NormalizeURLSlice(thisStep.URLTargets)
			targets = tools.UniqueStrings(append(targets, urltools.ExtractURLs(weightedURLs)...))
			if len(targets) > promptschema.MAX_URLS_PER_STEP {
				targets = targets[:promptschema.MAX_URLS_PER_STEP]
			}
			if len(targets) > 0 {
				tools.ProcessURLs(targets, &ctx, &allKnowledge, allURLs, &visitedURLs, &badURLs, schemaGen, currentQuestion)
			}
			allowRead = false

		case "coding":
			sandbox := tools.NewCodeSandbox(types.Memory{AllContext: ctx.ActionTracker.AllContext(), URLs: weightedURLs[:tools.Min(20, len(weightedURLs))], AllKnowledge: allKnowledge}, &ctx, schemaGen)
			if sol, err := sandbox.Solve(thisStep.CodingIssue); err == nil {
				allKnowledge = append(allKnowledge, tools.MakeCodingKnowledge(thisStep.CodingIssue, sol))
			}
			allowCoding = false
		}

		// --- 持久化 ----
		tools.StoreContext(systemPrompt, currentSchema, types.Memory{
			AllContext:   ctx.ActionTracker.AllContext(),
			AllKeywords:  allKeywords,
			AllQuestions: allQuestions,
			AllKnowledge: allKnowledge,
			WeightedURLs: weightedURLs,
			MsgWithK:     msgWithKnowledge,
		}, totalStep)

		time.Sleep(time.Duration(config.STEP_SLEEP) * time.Millisecond)
		if ans, ok := thisStep.(types.AnswerAction); ok && ans.IsFinal {
			break
		}
	}

	// ------------ Beast Mode & Markdown 后处理 ------------

	// 若尚未得到最终答案，进入 Beast Mode（最后一搏）
	if ans, ok := thisStep.(types.AnswerAction); !ok || !ans.IsFinal {
		step++
		totalStep++

		// Beast Mode Prompt
		systemPrompt = prompts.GetPrompt(
			diaryContext,
			allQuestions,
			allKeywords,
			false, // allowReflect
			false, // allowAnswer
			false, // allowRead
			false, // allowSearch
			false, // allowCoding
			allKnowledge,
			weightedURLs,
			true, // beastMode = true
		)

		// Beast Schema：仅允许 answer
		currentSchema = schemaGen.GetAgentSchema(false, false, true, false, false, question)
		msgWithKnowledge = prompts.ComposeMsgs(messages, allKnowledge, question, finalAnswerPIP)

		beastRes, _ := generator.GenerateObject(tools.GenerationRequest{
			Model:      "agentBeastMode",
			Schema:     currentSchema,
			System:     systemPrompt,
			Messages:   msgWithKnowledge,
			NumRetries: 2,
		})
		thisStep = tools.ParseStepAction(beastRes.Object)
		tools.UpdateReferences(&thisStep, allURLs)
		if ans, ok := thisStep.(types.AnswerAction); ok {
			ans.IsFinal = true
			thisStep = ans
		}
		ctx.ActionTracker.TrackAction(types.TrackedAction{TotalStep: totalStep, Step: thisStep, Gaps: gaps})
	}

	// ---------- Markdown 生成 ----------
	if ans, ok := thisStep.(types.AnswerAction); ok {
		if !trivialQuestion {
			ans.MdAnswer = tools.ConvertHtmlTablesToMd(
				tools.FixBadURLMdLinks(
					tools.FixCodeBlockIndentation(
						tools.RepairMarkdownFootnotesOuter(
							tools.FixMarkdown(
								tools.BuildMdFromAnswer(ans),
								allKnowledge,
								&ctx,
								schemaGen,
							),
						),
						allURLs,
					),
				),
			)
		} else {
			ans.MdAnswer = tools.ConvertHtmlTablesToMd(
				tools.FixCodeBlockIndentation(
					tools.BuildMdFromAnswer(ans),
				),
			)
		}
		thisStep = ans
	}

	// ---------- 最终持久化 ----------

	tools.StoreContext(systemPrompt, currentSchema, types.Memory{
		AllContext:   ctx.ActionTracker.AllContext(),
		AllKeywords:  allKeywords,
		AllQuestions: allQuestions,
		AllKnowledge: allKnowledge,
		WeightedURLs: weightedURLs,
		MsgWithK:     msgWithKnowledge,
	}, totalStep)

	// ---------- 结果汇总 ----------
	returned := urltools.Take(weightedURLs, numReturnedURLs)
	readURLs := tools.Filter(returned, func(u string) bool { return !tools.Contains(badURLs, u) })
	allRet := urltools.Map(weightedURLs, func(b types.BoostedSearchSnippet) string { return b.URL })

	return thisStep, ctx, returned, readURLs, allRet
}

// ────────────────────────调试持久化（storeContext）───────────────────
type Memory struct {
	AllContext   []types.StepAction
	AllKeywords  []string
	AllQuestions []string
	AllKnowledge []types.KnowledgeItem
	WeightedURLs []types.BoostedSearchSnippet
	MsgWithK     []ai.CoreMessage
}

// storeContext —— 与 TS 版等价，且不依赖任何辅助函数。
func storeContext(prompt string, schema any, m Memory, step int) {
	// 若采用异步本地上下文，直接返回（保持与旧 Go 版逻辑一致）
	if os.Getenv("ASYNC_LOCAL") == "1" {
		return
	}

	// 写入 prompt-<step>.txt
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
	_ = os.WriteFile(
		fmt.Sprintf("prompt-%d.txt", step),
		[]byte(fmt.Sprintf("Prompt:\n%s\n\nJSONSchema:\n%s", prompt, schemaJSON)),
		0644,
	)

	// 内联函数：序列化并落盘
	write := func(path string, v any) {
		b, _ := json.MarshalIndent(v, "", "  ")
		_ = os.WriteFile(path, b, 0644)
	}

	write("context.json", m.AllContext)
	write("queries.json", m.AllKeywords)
	write("questions.json", m.AllQuestions)
	write("knowledge.json", m.AllKnowledge)
	write("urls.json", m.WeightedURLs)
	write("messages.json", m.MsgWithK)
}

// ───────────────────────── CLI 入口 ─────────────────────────
func Main() {
	q := ""
	if len(os.Args) > 1 {
		q = os.Args[1]
	}
	res, ctx, visited, _, _, err := GetResponse(
		q, 1_000_000, 2, nil, nil, 100,
		false, nil, nil, nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Final Answer:", res.AsAnswer().Answer)
	fmt.Println("Visited URLs:", visited)
	ctx.TokenTracker.PrintSummary()
}
