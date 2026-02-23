import React, {useEffect, useCallback} from 'react';
import styles from './styles.module.css';

interface GitHubRepoModalProps {
  onClose: () => void;
}

export default function GitHubRepoModal({onClose}: GitHubRepoModalProps) {
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    },
    [onClose],
  );

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) onClose();
  };

  return (
    <div className={styles.backdrop} onClick={handleBackdropClick}>
      <div className={styles.modal} role="dialog" aria-modal="true" aria-labelledby="gh-modal-title">
        <div className={styles.icon}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
        </div>
        <h2 id="gh-modal-title" className={styles.title}>Repository Not Yet Public</h2>
        <p className={styles.message}>
          The OpenPact repository will be open-sourced soon. Interested in early
          access or beta testing? Reach out at{' '}
          <a href="mailto:hello@openpact.ai" className={styles.emailLink}>
            hello@openpact.ai
          </a>
        </p>
        <button className={styles.closeButton} onClick={onClose}>
          Got it
        </button>
      </div>
    </div>
  );
}
