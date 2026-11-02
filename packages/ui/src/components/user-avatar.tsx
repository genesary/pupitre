import { useState } from 'react';

import { isEmoji, safeUrl } from '../lib/security';
import { cn } from 'cn';

type UserAvatarProps = {
  username: string;
  avatarUrl: string | null | undefined;
  /** Class of the <img> (pupitre: nav-avatar, avatar-circle…). */
  imageClassName?: string;
  /** Class of the initial or emoji bubble (pupitre: nav-avatar-initial, avatar-initial…). */
  fallbackClassName?: string;
  /** Extra class when the bubble holds an emoji. */
  emojiClassName?: string;
};

/**
 * Renders an avatar from an untrusted `avatarUrl` (spec 003 S3): an https image,
 * a single emoji as text, or the username's initial. A value is never parsed as
 * HTML, which closes audit finding C4. Defaults to the top bar look.
 */
export function UserAvatar({
  username,
  avatarUrl,
  imageClassName = 'nav-avatar',
  fallbackClassName = 'nav-avatar-initial',
  emojiClassName = 'text-[1.2rem]',
}: UserAvatarProps) {
  const [failed, setFailed] = useState(false);
  const initial = (username.trim()[0] ?? '?').toUpperCase();
  const value = avatarUrl?.trim() ?? '';
  const imageUrl = value.startsWith('https://') ? safeUrl(value) : null;

  if (imageUrl && !failed) {
    return (
      <img
        src={imageUrl}
        alt=""
        referrerPolicy="no-referrer"
        className={imageClassName}
        onError={() => setFailed(true)}
      />
    );
  }

  if (value && isEmoji(value)) {
    return (
      <span className={cn(fallbackClassName, emojiClassName)} aria-hidden="true">
        {value}
      </span>
    );
  }

  return (
    <span className={fallbackClassName} aria-hidden="true">
      {initial}
    </span>
  );
}
