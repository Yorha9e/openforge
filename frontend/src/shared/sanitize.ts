import DOMPurify from 'dompurify';

const ALLOWED_TAGS = ['h1','h2','h3','h4','h5','h6','p','code','pre','ul','ol','li','strong','em','a','table','thead','tbody','tr','th','td','blockquote','br','hr'];
const ALLOWED_ATTR = ['href','target','rel'];

export function sanitizeHTML(dirty: string): string {
  return DOMPurify.sanitize(dirty, { ALLOWED_TAGS, ALLOWED_ATTR, ALLOW_DATA_ATTR: false });
}
