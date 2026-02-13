import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8081/api';

export const api = axios.create({
  baseURL: API_URL,
});

export interface Job {
  id: string;
  topic: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  report?: string;
  created_at: string;
  updated_at: string;
}

export interface LogEntry {
  id: number;
  timestamp: string;
  level: string;
  message: string;
  metadata: Record<string, any>;
}

export const createJob = async (topic: string) => {
  const { data } = await api.post<Job>('/research', { topic });
  return data;
};

export const getJobs = async () => {
  const { data } = await api.get<Job[]>('/research');
  return data;
};

export const getJob = async (id: string) => {
  const { data } = await api.get<Job>(`/research/${id}`);
  return data;
};

export const getJobLogs = async (id: string) => {
  const { data } = await api.get<LogEntry[]>(`/research/${id}/logs`);
  return data;
};

// Chat API

export interface Conversation {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: string;
  conversation_id: string;
  role: 'user' | 'model';
  content: string;
  created_at: string;
}

export const createConversation = async () => {
  const { data } = await api.post<Conversation>('/chat/conversations');
  return data;
};

export const getConversations = async () => {
  const { data } = await api.get<Conversation[]>('/chat/conversations');
  return data;
};

export const getMessages = async (id: string) => {
  const { data } = await api.get<Message[]>(`/chat/conversations/${id}/messages`);
  return data;
};

export const sendMessage = async (id: string, content: string) => {
  const { data } = await api.post<Message>(`/chat/conversations/${id}/messages`, { content });
  return data;
};
