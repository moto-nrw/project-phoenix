// components/dashboard/notification-center.tsx
"use client";

import React, { useState, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';

// Notification interfaces
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

interface NotificationCenterProps {
  className?: string;
}

// Notification type icons
const getNotificationIcon = (type: string) => {
  switch (type) {
    case 'success':
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      );
    case 'warning':
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z" />
        </svg>
      );
    case 'error':
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
        </svg>
      );
    case 'activity':
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
      );
    default: // info
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      );
  }
};

// Notification type styles
const getNotificationStyles = (type: string, isRead: boolean) => {
  const baseStyles = isRead ? 'opacity-60' : '';
  
  switch (type) {
    case 'success':
      return `${baseStyles} bg-green-50 border-green-200 text-green-800`;
    case 'warning':
      return `${baseStyles} bg-amber-50 border-amber-200 text-amber-800`;
    case 'error':
      return `${baseStyles} bg-red-50 border-red-200 text-red-800`;
    case 'activity':
      return `${baseStyles} bg-blue-50 border-blue-200 text-blue-800`;
    default: // info
      return `${baseStyles} bg-gray-50 border-gray-200 text-gray-800`;
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

// Mock notifications - replace with real data
const mockNotifications: Notification[] = [
  {
    id: '1',
    type: 'warning',
    title: 'Schüler nicht eingecheckt',
    message: 'Max Mustermann (4a) hat sich heute noch nicht eingecheckt.',
    timestamp: new Date(Date.now() - 300000), // 5 minutes ago
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
    timestamp: new Date(Date.now() - 900000), // 15 minutes ago
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
    timestamp: new Date(Date.now() - 1800000), // 30 minutes ago
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
    timestamp: new Date(Date.now() - 7200000), // 2 hours ago
    isRead: true,
    priority: 'medium',
    category: 'activity',
    actionUrl: '/substitutions',
    metadata: { teacherName: 'Frau Müller' }
  }
];

export function NotificationCenter({ className = '' }: NotificationCenterProps) {
  const [notifications, setNotifications] = useState<Notification[]>(mockNotifications);
  const [isOpen, setIsOpen] = useState(false);
  const [filter, setFilter] = useState<'all' | 'unread'>('all');
  const dropdownRef = useRef<HTMLDivElement>(null);
  const router = useRouter();

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [isOpen]);

  // Calculate notification counts
  const unreadCount = notifications.filter(n => !n.isRead).length;
  const highPriorityUnread = notifications.filter(n => !n.isRead && n.priority === 'high').length;

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
    setIsOpen(false);
  };

  // Toggle dropdown
  const toggleDropdown = () => {
    setIsOpen(!isOpen);
  };

  return (
    <div className={`relative ${className}`}>
      {/* Notification Bell Button */}
      <button
        onClick={toggleDropdown}
        className={`relative p-2 rounded-lg transition-all duration-200 ${
          isOpen 
            ? 'bg-[#5080d8]/10 text-[#5080d8]' 
            : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'
        }`}
        aria-label={`${unreadCount} ungelesene Benachrichtigungen`}
      >
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
        </svg>
        
        {/* Notification Dot */}
        {unreadCount > 0 && (
          <div className={`absolute top-1.5 right-1.5 w-2 h-2 rounded-full ${
            highPriorityUnread > 0 ? 'bg-red-500' : 'bg-[#5080d8]'
          }`}></div>
        )}
      </button>

      {/* Notification Dropdown */}
      {isOpen && (
        <div 
          ref={dropdownRef}
          className="absolute right-0 top-full mt-2 w-96 bg-white rounded-xl shadow-xl border border-gray-200 z-50 max-h-96 overflow-hidden"
        >
          {/* Header */}
          <div className="px-4 py-3 border-b border-gray-100 bg-gray-50/50">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900">Benachrichtigungen</h3>
              {unreadCount > 0 && (
                <button
                  onClick={markAllAsRead}
                  className="text-xs text-[#5080d8] hover:text-[#5080d8]/80 font-medium"
                >
                  Alle als gelesen markieren
                </button>
              )}
            </div>
            
            {/* Filter Tabs */}
            <div className="flex space-x-1 mt-2">
              <button
                onClick={() => setFilter('all')}
                className={`px-3 py-1 rounded-md text-xs font-medium transition-colors duration-200 ${
                  filter === 'all' 
                    ? 'bg-[#5080d8] text-white' 
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                }`}
              >
                Alle ({notifications.length})
              </button>
              <button
                onClick={() => setFilter('unread')}
                className={`px-3 py-1 rounded-md text-xs font-medium transition-colors duration-200 ${
                  filter === 'unread' 
                    ? 'bg-[#5080d8] text-white' 
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                }`}
              >
                Ungelesen ({unreadCount})
              </button>
            </div>
          </div>

          {/* Notifications List */}
          <div className="max-h-80 overflow-y-auto">
            {filteredNotifications.length > 0 ? (
              <div className="py-2">
                {filteredNotifications.map((notification) => (
                  <button
                    key={notification.id}
                    onClick={() => handleNotificationClick(notification)}
                    className={`w-full px-4 py-3 text-left hover:bg-gray-50 active:bg-gray-100 transition-colors duration-150 border-l-3 ${
                      notification.isRead 
                        ? 'border-transparent' 
                        : notification.priority === 'high' 
                          ? 'border-red-500' 
                          : 'border-[#5080d8]'
                    }`}
                  >
                    <div className="flex items-start space-x-3">
                      {/* Icon */}
                      <div className={`mt-0.5 p-1.5 rounded-lg ${getNotificationStyles(notification.type, notification.isRead)}`}>
                        {getNotificationIcon(notification.type)}
                      </div>
                      
                      {/* Content */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-start justify-between">
                          <h4 className={`text-sm font-medium truncate ${
                            notification.isRead ? 'text-gray-600' : 'text-gray-900'
                          }`}>
                            {notification.title}
                          </h4>
                          <span className="text-xs text-gray-400 flex-shrink-0 ml-2">
                            {formatTimeAgo(notification.timestamp)}
                          </span>
                        </div>
                        
                        <p className={`text-xs mt-1 line-clamp-2 ${
                          notification.isRead ? 'text-gray-400' : 'text-gray-600'
                        }`}>
                          {notification.message}
                        </p>
                        
                        {/* Priority Indicator */}
                        {!notification.isRead && notification.priority === 'high' && (
                          <div className="flex items-center mt-2">
                            <div className="w-1.5 h-1.5 bg-red-500 rounded-full mr-1"></div>
                            <span className="text-xs text-red-600 font-medium">Hohe Priorität</span>
                          </div>
                        )}
                      </div>
                      
                      {/* Unread Indicator */}
                      {!notification.isRead && (
                        <div className="w-2 h-2 bg-[#5080d8] rounded-full mt-2 flex-shrink-0"></div>
                      )}
                    </div>
                  </button>
                ))}
              </div>
            ) : (
              /* Empty State */
              <div className="px-4 py-8 text-center">
                <div className="w-12 h-12 mx-auto mb-3 rounded-full bg-gray-100 flex items-center justify-center">
                  <svg className="w-6 h-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                  </svg>
                </div>
                <h3 className="text-sm font-medium text-gray-900 mb-1">
                  {filter === 'unread' ? 'Keine ungelesenen Benachrichtigungen' : 'Keine Benachrichtigungen'}
                </h3>
                <p className="text-xs text-gray-500">
                  {filter === 'unread' 
                    ? 'Alle Benachrichtigungen sind gelesen.' 
                    : 'Neue Benachrichtigungen werden hier angezeigt.'
                  }
                </p>
              </div>
            )}
          </div>

          {/* Footer */}
          {filteredNotifications.length > 0 && (
            <div className="px-4 py-3 border-t border-gray-100 bg-gray-50/50">
              <button
                onClick={() => {
                  setIsOpen(false);
                  router.push('/notifications'); // Assuming a notifications page exists
                }}
                className="w-full text-center text-xs text-[#5080d8] hover:text-[#5080d8]/80 font-medium"
              >
                Alle Benachrichtigungen anzeigen
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}