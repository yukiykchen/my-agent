/**
 * Frontmatter 解析工具
 */

import matter from 'gray-matter';

export interface FrontmatterResult<T = Record<string, any>> {
  data: T;
  content: string;
}

/**
 * 解析 Markdown 文件的 frontmatter
 */
export function parseFrontmatter<T = Record<string, any>>(
  text: string
): FrontmatterResult<T> {
  const { data, content } = matter(text);
  return {
    data: data as T,
    content: content.trim()
  };
}

/**
 * 生成带 frontmatter 的 Markdown
 */
export function stringifyFrontmatter(
  data: Record<string, any>,
  content: string
): string {
  return matter.stringify(content, data);
}
