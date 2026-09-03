// 与后端 api/openapi.yaml 契约对应的最小手写 DTO（后端 camelCase 字段 + snake_case 枚举）。

export type Role = 'learner' | 'admin'
export type SessionStatus = 'active' | 'grading' | 'completed' | 'analysis_failed'
export type QuestionType = 'single_choice' | 'multiple_choice' | 'fill_blank' | 'short_answer'
export type GradingStatus = 'correct' | 'incorrect' | 'unanswered' | 'pending' | 'failed'
export type AnswerAuthority = 'official' | 'human_verified'
export type ExplanationSource = 'official' | 'human_verified' | 'ai'

export interface Me {
  id: string
  email: string
  role: Role
  defaultLevelId: string | null
}

// ---- catalog ----

export interface Level {
  id: string
  code: string
  name: string
}

export interface Subject {
  id: string
  code: string
  name: string
}

export interface Exam {
  id: string
  code: string
  name: string
  levels: Level[]
  subjects: Subject[]
}

export interface PracticeSource {
  id: string
  name: string
  questionCount: number
  sections: PracticeSourceSection[]
}

export interface PracticeSourceSection {
  id: string
  name: string
  questionCount: number
}

// ---- knowledge ----

export interface KPStats {
  confirmedAnswered: number
  confirmedCorrect: number
  recentAnswered: number
  recentCorrect: number
  aiAnswered: number
  aiCorrect: number
  consecutiveWrong: number
  lastPracticedAt?: string
}

export interface KnowledgePointItem {
  id: string
  name: string
  levelId: string
  levelCode: string
  subjectId: string
  subjectName: string
  parentId: string | null
  questionCount: number
  stats?: KPStats
}

export interface KnowledgePointDetail extends KnowledgePointItem {
  description: string
  commonMistakes: string
  examples: string
  status: string
}

// ---- practice：答题前 DTO（契约保证不含答案字段） ----

export interface OptionDTO {
  id: string
  label: string
  text: string
}

export interface MaterialDTO {
  id: string
  title?: string
  content: string
}

export type AnswerValue = { optionIds: string[] } | { text: string } | null

export interface PreSubmitItem {
  id: string
  position: number
  type: QuestionType
  material: MaterialDTO | null
  stem: string
  sourceSectionName?: string
  options: OptionDTO[]
  savedAnswer: AnswerValue
  markedForReview: boolean
  savedAt: string | null
}

export interface PreSubmitSession {
  id: string
  status: SessionStatus
  answeredCount: number
  totalCount: number
  items: PreSubmitItem[]
}

// ---- practice：答题后 DTO ----

export interface ExplanationDTO {
  text: string
  source: ExplanationSource
}

export interface KPRef {
  id: string
  name: string
}

export interface ResultItem {
  id: string
  position: number
  type: QuestionType
  material: MaterialDTO | null
  stem: string
  sourceSectionName?: string
  options: OptionDTO[]
  knowledgePoints: KPRef[]
  userAnswer: AnswerValue
  gradingStatus: GradingStatus
  gradingSource: 'deterministic' | 'ai' | null
  answerAuthority: AnswerAuthority | null
  correctAnswer: AnswerValue
  explanation: ExplanationDTO | null
}

export interface ResultSummary {
  confirmed: { correct: number; total: number; accuracy: number | null }
  ai: { correct: number; completed: number; pending: number; failed: number }
}

export interface AIAnalysis {
  status: 'not_requested' | 'pending' | 'completed' | 'failed'
  text: string
}

export interface ResultSession {
  id: string
  status: SessionStatus
  createdAt: string
  submittedAt: string | null
  summary: ResultSummary
  aiAnalysis: AIAnalysis
  items: ResultItem[]
}

export interface SessionListItem {
  id: string
  status: SessionStatus
  totalCount: number
  createdAt: string
  submittedAt: string | null
}

// ---- learning ----

export interface ActiveSessionDTO {
  id: string
  answeredCount: number
  totalCount: number
}

export interface RecentSessionDTO {
  id: string
  status: SessionStatus
  totalCount: number
  createdAt: string
  submittedAt: string | null
}

export interface Recommendation {
  type: 'knowledge' | 'comprehensive'
  knowledgePointId?: string | null
  name: string
  recentAnswered: number
  recentWrongCount: number
  accuracy?: number | null
  consecutiveWrong: number
  suggestedCount: number
  reason: string
  knowledgePointIds: string[]
}

export interface DashboardDTO {
  activeSession: ActiveSessionDTO | null
  recentSessions: RecentSessionDTO[]
  recommendations: Recommendation[]
  comprehensive?: Recommendation | null
  statsEmpty: boolean
  memory: LearningMemoryDTO
}

export interface LearningMemoryDTO {
  confirmedAnswered: number
  confirmedCorrect: number
  aiAnswered: number
  aiCorrect: number
  statsUpdatedAt?: string | null
  advice: {
    status: 'not_requested' | 'pending' | 'completed' | 'failed'
    text: string
    updatedAt?: string | null
  }
}

export interface WrongItem {
  itemId: string
  sessionId: string
  questionId: string
  position: number
  type: QuestionType
  stem: string
  options: OptionDTO[] | null
  material?: MaterialDTO
  knowledgePoints: KPRef[]
  gradingStatus: string
  answerAuthority?: string | null
  userAnswer: AnswerValue
  correctAnswer: AnswerValue
  explanation?: ExplanationDTO
}

// ---- admin ----

export interface SourceSectionDTO {
  id: string
  sourceId: string
  name: string
}

export interface SourceDTO {
  id: string
  name: string
  kind: 'book' | 'past_exam' | 'self_made' | 'ai_generated'
  author: string
  publisher: string
  year: number | null
  licenseNote: string
  internalNote: string
  sections: SourceSectionDTO[]
}

export interface AnswerKeyDTO {
  value: Record<string, unknown>
  authority: AnswerAuthority
  explanation: string
}

export interface QuestionVersionDTO {
  id: string
  questionId: string
  versionNo: number
  type: QuestionType
  stem: string
  materialTitle?: string
  materialContent?: string
  options: OptionDTO[] | null
  levelId: string
  subjectId: string
  sourceSectionId?: string | null
  difficulty: number
  knowledgePointIds: string[]
  answerKey?: AnswerKeyDTO
  createdAt: string
}

export interface QuestionAdminDTO {
  id: string
  status: 'draft' | 'in_review' | 'published' | 'retired'
  hasAnswer: boolean
  publishedVersionId: string | null
  currentVersion: QuestionVersionDTO | null
  publishedAt?: string | null
  retiredAt?: string | null
  updatedAt: string
}

export interface OverviewDTO {
  draft: number
  inReview: number
  published: number
  retired: number
  publishedNoKnowledge: number
  publishedNoSource: number
  publishedNoAnswer: number
  openIssues: number
}

export interface AdminKnowledgePoint {
  id: string
  examId: string
  levelId: string
  subjectId: string
  parentId: string | null
  name: string
  description: string
  commonMistakes: string
  examples: string
  status: 'draft' | 'published' | 'retired'
  questionCount: number
}

export interface IssueReportDTO {
  id: string
  targetType: string
  description: string
  status: 'open' | 'resolved' | 'dismissed'
  resolutionNote: string
  createdAt: string
  userEmail?: string
  stem?: string
  questionId: string
}

// ---- admin imports ----

export interface ImportJobDTO {
  id: string
  fileName: string
  mimeType: string
  sizeBytes: number
  status: 'uploaded' | 'extracting' | 'structuring' | 'review_ready' | 'published' | 'failed'
  stageError: string
  extractedText?: string
  itemCount: number
  createdAt: string
  updatedAt: string
}

export interface ImportAnswerDTO {
  value: Record<string, unknown>
  authority: AnswerAuthority
  explanation: string
}

export interface ImportSuggestionDTO {
  value: Record<string, unknown>
  explanation: string
}

export interface ImportDraftDTO {
  materialKey?: string
  type: QuestionType
  stem: string
  options: OptionDTO[]
  materialTitle?: string
  materialContent?: string
  levelId: string
  subjectId: string
  sourceSectionId?: string | null
  difficulty: number
  knowledgePointIds: string[]
  answer?: ImportAnswerDTO
  sourceAnswer?: ImportAnswerDTO
  aiSuggestedAnswer?: ImportSuggestionDTO
}

export interface ImportItemDTO {
  id: string
  jobId: string
  position: number
  rawExcerpt: string
  draft: ImportDraftDTO | null
  anomalies: string[]
  reviewStatus: 'pending' | 'approved' | 'published' | 'rejected'
  publishedQuestionId: string | null
  jobStatus: ImportJobDTO['status']
  createdAt: string
  updatedAt: string
}
