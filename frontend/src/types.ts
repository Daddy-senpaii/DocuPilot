export const API_BASE_URL = 'http://localhost:8080';

export interface User {
  id: number;
  email: string;
  created_at: string;
}

export interface DocumentItem {
  id: number;
  filename: string;
  char_count: number;
  status: string;
  created_at: string;
}

export interface QAItem {
  id: number;
  question: string;
  answer: string;
  created_at: string;
}

export interface ConversionPair {
  source_format: string;
  target_format: string;
  title: string;
  description: string;
  icon: string;
}

export interface ConversionResult {
  filename: string;
  blob: Blob;
  blobUrl: string;
  size: number;
  targetFormat: string;
  textPreview?: string;
}

export type ActiveTab = 'upload' | 'paste' | 'library' | 'converter';

export const DEFAULT_CONVERSION_PAIRS: ConversionPair[] = [
  { source_format: 'pdf', target_format: 'docx', title: 'PDF to Word', description: 'Convert PDF into editable Word document (.docx)', icon: '📄 ➡️ 📝' },
  { source_format: 'docx', target_format: 'pdf', title: 'Word to PDF', description: 'Render Word document (.docx) into standard PDF', icon: '📝 ➡️ 📄' },
  { source_format: 'pdf', target_format: 'xlsx', title: 'PDF to Excel', description: 'Convert PDF tables into Excel spreadsheet (.xlsx)', icon: '📄 ➡️ 📊' },
  { source_format: 'xlsx', target_format: 'pdf', title: 'Excel to PDF', description: 'Render Excel spreadsheet (.xlsx) into clean PDF', icon: '📊 ➡️ 📄' },
  { source_format: 'pdf', target_format: 'pptx', title: 'PDF to PowerPoint', description: 'Transform PDF pages into presentation slides (.pptx)', icon: '📄 ➡️ 📽️' },
  { source_format: 'pptx', target_format: 'pdf', title: 'PowerPoint to PDF', description: 'Render presentation slides (.pptx) into standard PDF deck', icon: '📽 ➡️ 📄' },
  { source_format: 'docx', target_format: 'txt', title: 'Word to Text', description: 'Extract plain text from Word document (.docx)', icon: '📝 ➡️ 📋' },
  { source_format: 'txt', target_format: 'pdf', title: 'Text to PDF', description: 'Typeset plain text (.txt) into paginated PDF document', icon: '📋 ➡️ 📄' },
];

export const QUICK_PROMPTS = [
  'Summarize the key takeaways in 3 bullets',
  'What are the main findings or recommendations?',
  'Extract any important dates, numbers, or action items',
];
