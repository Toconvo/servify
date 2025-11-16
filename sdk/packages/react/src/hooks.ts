import { useState, useCallback } from 'react';
import { useServify } from './ServifyProvider';
import { Ticket, CustomerSatisfaction } from '@servify/core';

export interface UseTicketsReturn {
  tickets: Ticket[];
  isLoading: boolean;
  error: Error | null;
  createTicket: (data: {
    title: string;
    description: string;
    priority?: 'low' | 'normal' | 'high' | 'urgent';
    category: string;
  }) => Promise<Ticket>;
  loadTickets: () => Promise<void>;
}

export function useTickets(): UseTicketsReturn {
  const { sdk } = useServify();
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const createTicket = useCallback(async (data: {
    title: string;
    description: string;
    priority?: 'low' | 'normal' | 'high' | 'urgent';
    category: string;
  }) => {
    if (!sdk) {
      throw new Error('SDK not initialized');
    }

    try {
      setError(null);
      const ticket = await sdk.createTicket(data);
      setTickets(prev => [ticket, ...prev]);
      return ticket;
    } catch (err) {
      setError(err as Error);
      throw err;
    }
  }, [sdk]);

  const loadTickets = useCallback(async () => {
    if (!sdk) {
      throw new Error('SDK not initialized');
    }

    const customer = sdk.getCustomer();
    if (!customer) {
      throw new Error('Customer not found');
    }

    try {
      setIsLoading(true);
      setError(null);
      // 这里需要实现获取客户工单的方法
      // const result = await sdk.getCustomerTickets(customer.id);
      // setTickets(result);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [sdk]);

  return {
    tickets,
    isLoading,
    error,
    createTicket,
    loadTickets,
  };
}

export interface UseAIReturn {
  isLoading: boolean;
  error: Error | null;
  askAI: (question: string) => Promise<{ answer: string; confidence: number }>;
}

export function useAI(): UseAIReturn {
  const { sdk } = useServify();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const askAI = useCallback(async (question: string) => {
    if (!sdk) {
      throw new Error('SDK not initialized');
    }

    try {
      setIsLoading(true);
      setError(null);
      const result = await sdk.askAI(question);
      return result;
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [sdk]);

  return {
    isLoading,
    error,
    askAI,
  };
}

export interface UseSatisfactionReturn {
  isLoading: boolean;
  error: Error | null;
  submitRating: (data: {
    ticketId?: number;
    rating: number;
    comment?: string;
    category?: string;
  }) => Promise<CustomerSatisfaction>;
}

export function useSatisfaction(): UseSatisfactionReturn {
  const { sdk } = useServify();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const submitRating = useCallback(async (data: {
    ticketId?: number;
    rating: number;
    comment?: string;
    category?: string;
  }) => {
    if (!sdk) {
      throw new Error('SDK not initialized');
    }

    try {
      setIsLoading(true);
      setError(null);
      const result = await sdk.submitSatisfaction({
        ticket_id: data.ticketId,
        rating: data.rating,
        comment: data.comment,
        category: data.category,
      });
      return result;
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [sdk]);

  return {
    isLoading,
    error,
    submitRating,
  };
}