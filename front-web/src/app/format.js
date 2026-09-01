// 统一格式化：同一页面内精度与风格保持一致。
const percentFmt = new Intl.NumberFormat('zh-CN', {
    style: 'percent',
    maximumFractionDigits: 1,
});
const dateTimeFmt = new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
});
export function formatPercent(value) {
    if (value === null || value === undefined) {
        return '—';
    }
    return percentFmt.format(value);
}
export function formatDateTime(value) {
    if (!value) {
        return '—';
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return '—';
    }
    return dateTimeFmt.format(date);
}
export function formatTime(value) {
    if (!value) {
        return '—';
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return '—';
    }
    return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(date);
}
export const sessionStatusText = {
    active: '答题中',
    grading: '判分中',
    completed: '已完成',
    analysis_failed: '部分分析失败',
};
export const gradingStatusText = {
    correct: '正确',
    incorrect: '错误',
    unanswered: '未作答',
    pending: 'AI 判定中',
    failed: '分析失败',
};
export const authorityText = {
    official: '官方答案',
    human_verified: '已审核答案',
};
export const explanationSourceText = {
    official: '官方解析',
    human_verified: '人工解析',
    ai: 'AI 解析（可能有误）',
};
export const questionTypeText = {
    single_choice: '单选',
    multiple_choice: '多选',
    fill_blank: '填空',
    short_answer: '简答',
};
export const sourceKindText = {
    book: '书籍',
    past_exam: '真题',
    self_made: '自建',
    ai_generated: 'AI 生成',
};
export const statusText = {
    draft: '草稿',
    in_review: '待审核',
    published: '已发布',
    retired: '已下架',
    open: '待处理',
    resolved: '已解决',
    dismissed: '已驳回',
};
