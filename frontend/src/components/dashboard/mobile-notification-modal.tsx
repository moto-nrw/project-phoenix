// components/dashboard/mobile-notification-modal.tsx
"use client";

import React, { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { createPortal } from 'react-dom';
import { useModal } from './modal-context';

// Notification interfaces (copied from notification-center.tsx for mobile use)
interface Notification {
  id: string;
  type: 'info' | 'success' | 'warning' | 'error' | 'activity';
  title: string;
  message: string;
  timestamp: Date;
  isRead: boolean;
  actionUrl?: string;
  priority: 'low' | 'medium' | 'high';
  category: 'student' | 'room' | 'activity' | 'system' | 'security';
  metadata?: {
    studentName?: string;
    roomName?: string;
    activityName?: string;
    teacherName?: string;
  };
}

interface MobileNotificationModalProps {
  isOpen: boolean;
  onClose: () => void;
}

// Mobile-optimized notification icons
const getMobileNotificationIcon = (type: string) => {
  switch (type) {
    case 'success':
      return (
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      );
    case 'warning':
      return (
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z" />
        </svg>
      );
    case 'error':
      return (
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
        </svg>
      );
    case 'activity':
      return (
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
      );
    default: // info
      return (
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      );
  }
};

// Mobile notification type styles - More subtle
const getMobileNotificationStyles = (type: string, isRead: boolean) => {
  const baseStyles = isRead ? 'opacity-60' : '';
  
  switch (type) {
    case 'success':
      return `${baseStyles} bg-green-100 text-green-700`;
    case 'warning':
      return `${baseStyles} bg-amber-100 text-amber-700`;
    case 'error':
      return `${baseStyles} bg-red-100 text-red-700`;
    case 'activity':
      return `${baseStyles} bg-blue-100 text-blue-700`;
    default: // info
      return `${baseStyles} bg-gray-100 text-gray-700`;
  }
};

// Time formatting utility
const formatTimeAgo = (date: Date): string => {
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  
  if (diffInSeconds < 60) {
    return 'Gerade eben';
  } else if (diffInSeconds < 3600) {
    const minutes = Math.floor(diffInSeconds / 60);
    return `vor ${minutes} min`;
  } else if (diffInSeconds < 86400) {
    const hours = Math.floor(diffInSeconds / 3600);
    return `vor ${hours}h`;
  } else {
    const days = Math.floor(diffInSeconds / 86400);
    return `vor ${days}d`;
  }
};

// Mock notifications for mobile
const mockMobileNotifications: Notification[] = [
  {
    id: '1',
    type: 'warning',
    title: 'Schüler nicht eingecheckt',
    message: 'Max Mustermann (4a) hat sich heute noch nicht eingecheckt.',
    timestamp: new Date(Date.now() - 300000),
    isRead: false,
    priority: 'high',
    category: 'student',
    actionUrl: '/students/1',
    metadata: { studentName: 'Max Mustermann' }
  },
  {
    id: '2',
    type: 'success',
    title: 'Neue Anmeldung',
    message: 'Emma Schmidt hat sich für die Fußball AG angemeldet.',
    timestamp: new Date(Date.now() - 900000),
    isRead: false,
    priority: 'medium',
    category: 'activity',
    actionUrl: '/activities/5',
    metadata: { studentName: 'Emma Schmidt', activityName: 'Fußball AG' }
  },
  {
    id: '3',
    type: 'info',
    title: 'Raum verfügbar',
    message: 'Turnhalle A ist jetzt wieder verfügbar.',
    timestamp: new Date(Date.now() - 1800000),
    isRead: true,
    priority: 'low',
    category: 'room',
    actionUrl: '/rooms/3',
    metadata: { roomName: 'Turnhalle A' }
  },
  {
    id: '5',
    type: 'activity',
    title: 'Vertretung eingeteilt',
    message: 'Frau Müller übernimmt heute die Gruppe Sonnenschein.',
    timestamp: new Date(Date.now() - 7200000),
    isRead: true,
    priority: 'medium',
    category: 'activity',
    actionUrl: '/substitutions',
    metadata: { teacherName: 'Frau Müller' }
  }
];

export function MobileNotificationModal({ isOpen, onClose }: MobileNotificationModalProps) {
  const [notifications, setNotifications] = useState<Notification[]>(mockMobileNotifications);
  const [filter, setFilter] = useState<'all' | 'unread'>('all');
  const modalRef = useRef<HTMLDivElement>(null);
  const router = useRouter();
  const { openModal, closeModal } = useModal();

  // Handle escape key and backdrop click
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    const handleClickOutside = (e: MouseEvent) => {
      if (modalRef.current && !modalRef.current.contains(e.target as Node)) {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown);
      document.addEventListener('mousedown', handleClickOutside);
      // Prevent body scroll when modal is open
      document.body.style.overflow = 'hidden';
      // Trigger blur effect on layout
      openModal();
      // Dispatch custom event for ResponsiveLayout
      window.dispatchEvent(new CustomEvent('mobile-modal-open'));
    } else {
      // Remove blur effect on layout
      closeModal();
      // Dispatch custom event for ResponsiveLayout
      window.dispatchEvent(new CustomEvent('mobile-modal-close'));
    }

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.removeEventListener('mousedown', handleClickOutside);
      document.body.style.overflow = 'unset';
    };
  }, [isOpen, onClose, openModal, closeModal]);

  // Calculate notification counts
  const unreadCount = notifications.filter(n => !n.isRead).length;

  // Filter notifications
  const filteredNotifications = notifications.filter(notification => {
    if (filter === 'unread') return !notification.isRead;
    return true;
  });

  // Mark notification as read
  const markAsRead = (notificationId: string) => {
    setNotifications(prev => 
      prev.map(notification => 
        notification.id === notificationId 
          ? { ...notification, isRead: true }
          : notification
      )
    );
  };

  // Mark all as read
  const markAllAsRead = () => {
    setNotifications(prev => 
      prev.map(notification => ({ ...notification, isRead: true }))
    );
  };

  // Handle notification click
  const handleNotificationClick = (notification: Notification) => {
    markAsRead(notification.id);
    if (notification.actionUrl) {
      router.push(notification.actionUrl);
    }
    onClose();
  };

  if (!isOpen) return null;

  const modalContent = (
    <div className="fixed inset-0 z-[9999] lg:hidden">
      {/* Backdrop without blur (blur is handled by ResponsiveLayout) */}
      <div className="fixed inset-0 bg-black/30 transition-all duration-300" />
      
      {/* Modal */}
      <div className="fixed inset-x-0 top-0 z-[9999]">
        <div 
          ref={modalRef}
          className="mx-4 mt-6 mb-safe bg-white rounded-xl shadow-lg border border-gray-200 overflow-hidden"
          style={{
            maxHeight: 'calc(100vh - 3rem - env(safe-area-inset-top) - env(safe-area-inset-bottom))'
          }}
        >
          {/* Header */}
          <div className="sticky top-0 bg-white border-b border-gray-100 z-10">
            <div className="flex items-center justify-between px-4 py-3">
              <div className="flex items-center space-x-2">
                <svg className="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                </svg>
                <div>
                  <h2 className="text-base font-medium text-gray-900">Benachrichtigungen</h2>
                  {unreadCount > 0 && (
                    <p className="text-xs text-gray-500">{unreadCount} ungelesen</p>
                  )}
                </div>
              </div>
              
              <button
                onClick={onClose}
                className="p-2 rounded-lg hover:bg-gray-100 active:bg-gray-200 transition-colors duration-200"
                aria-label="Benachrichtigungen schließen"
              >
                <svg className="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            
            {/* Mobile Filter Tabs - More subtle */}
            <div className="px-4 pb-3">
              <div className="flex space-x-1 bg-gray-100 rounded-lg p-1">
                <button
                  onClick={() => setFilter('all')}
                  className={`flex-1 px-3 py-1.5 rounded-md text-sm font-medium transition-all duration-200 ${
                    filter === 'all' 
                      ? 'bg-white text-gray-900 shadow-sm' 
                      : 'text-gray-600 hover:text-gray-900'
                  }`}
                >
                  Alle ({notifications.length})
                </button>
                <button
                  onClick={() => setFilter('unread')}
                  className={`flex-1 px-3 py-1.5 rounded-md text-sm font-medium transition-all duration-200 ${
                    filter === 'unread' 
                      ? 'bg-white text-gray-900 shadow-sm' 
                      : 'text-gray-600 hover:text-gray-900'
                  }`}
                >
                  Ungelesen ({unreadCount})
                </button>
              </div>
              
              {/* Mark all as read button - More subtle */}
              {unreadCount > 0 && (
                <button
                  onClick={markAllAsRead}
                  className="w-full mt-2 py-1.5 text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors duration-200"
                >
                  Alle als gelesen markieren
                </button>
              )}
            </div>
          </div>

          {/* Notifications List - More subtle */}
          <div className="overflow-y-auto" style={{ maxHeight: 'calc(100vh - 10rem)' }}>
            {filteredNotifications.length > 0 ? (
              <div className="px-4 py-2">
                {filteredNotifications.map((notification) => (
                  <button
                    key={notification.id}
                    onClick={() => handleNotificationClick(notification)}
                    className={`w-full p-3 mb-2 rounded-lg text-left transition-all duration-200 ${
                      !notification.isRead 
                        ? 'bg-blue-50/70 border border-blue-100 hover:bg-blue-50' 
                        : 'bg-gray-50/50 border border-transparent hover:bg-gray-100'
                    }`}
                  >
                    <div className="flex items-start space-x-3">
                      {/* Mobile Icon - Smaller and more subtle */}
                      <div className={`p-2 rounded-lg ${getMobileNotificationStyles(notification.type, notification.isRead)} flex-shrink-0`}>
                        {getMobileNotificationIcon(notification.type)}
                      </div>
                      
                      {/* Content */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-start justify-between mb-1">
                          <h3 className={`text-sm font-medium leading-tight ${
                            notification.isRead ? 'text-gray-600' : 'text-gray-900'
                          }`}>
                            {notification.title}
                          </h3>
                          <span className="text-xs text-gray-400 flex-shrink-0 ml-2">
                            {formatTimeAgo(notification.timestamp)}
                          </span>
                        </div>
                        
                        <p className={`text-xs leading-relaxed mb-2 ${
                          notification.isRead ? 'text-gray-500' : 'text-gray-600'
                        }`}>
                          {notification.message}
                        </p>
                        
                        {/* Priority and Status Indicators - More subtle */}
                        <div className="flex items-center justify-between">
                          <div className="flex items-center space-x-2">
                            {!notification.isRead && notification.priority === 'high' && (
                              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
                                Wichtig
                              </span>
                            )}
                          </div>
                          
                          {/* Action Indicator - More subtle */}
                          {notification.actionUrl && (
                            <svg className="w-3 h-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                            </svg>
                          )}
                        </div>
                      </div>
                      
                      {/* Unread Indicator - Smaller */}
                      {!notification.isRead && (
                        <div className="w-2 h-2 bg-blue-500 rounded-full flex-shrink-0 mt-1"></div>
                      )}
                    </div>
                  </button>
                ))}
              </div>
            ) : (
              /* Empty State - More subtle */
              <div className="px-6 py-8 text-center">
                <div className="w-12 h-12 mx-auto mb-3 rounded-lg bg-gray-100 flex items-center justify-center">
                  <svg className="w-6 h-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                  </svg>
                </div>
                <h3 className="text-sm font-medium text-gray-900 mb-1">
                  {filter === 'unread' ? 'Keine ungelesenen Benachrichtigungen' : 'Keine Benachrichtigungen'}
                </h3>
                <p className="text-xs text-gray-500">
                  {filter === 'unread' 
                    ? 'Alles erledigt!' 
                    : 'Hier erscheinen neue Updates.'
                  }
                </p>
              </div>
            )}
          </div>
          
          {/* Safe area padding */}
          <div className="h-safe-area-inset-bottom bg-gray-50" />
        </div>
      </div>
    </div>
  );

  // Render to body to avoid being affected by ResponsiveLayout blur
  if (typeof document !== 'undefined') {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
}