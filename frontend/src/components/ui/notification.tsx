import React from 'react';
import { Alert } from './alert';
import type { NotificationState } from '@/lib/use-notification';

interface NotificationProps {
  notification: NotificationState;
  className?: string;
}

export function Notification({ notification, className }: NotificationProps) {
  if (!notification.message) {
    return null;
  }

  return (
    <div className={className}>
      <Alert 
        type={notification.type} 
        message={notification.message} 
      />
    </div>
  );
}