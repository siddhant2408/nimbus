import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  fetchState,
  triggerRefresh,
  fetchPersonas,
  fetchAssignments,
  createPersona,
  updatePersona,
  deletePersona,
} from "@/api/client";
import type { Persona } from "@/types/api";

export function useStateQuery() {
  return useQuery({
    queryKey: ["state"],
    queryFn: fetchState,
    refetchInterval: 2000,
  });
}

export function usePersonasQuery() {
  return useQuery({
    queryKey: ["personas"],
    queryFn: fetchPersonas,
  });
}

export function useAssignmentsQuery() {
  return useQuery({
    queryKey: ["assignments"],
    queryFn: fetchAssignments,
  });
}

export function useRefreshMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: triggerRefresh,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["state"] });
    },
  });
}

export function useCreatePersonaMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (persona: Omit<Persona, "source_path">) =>
      createPersona(persona),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["personas"] });
    },
  });
}

export function useUpdatePersonaMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      persona,
    }: {
      name: string;
      persona: Omit<Persona, "source_path">;
    }) => updatePersona(name, persona),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["personas"] });
    },
  });
}

export function useDeletePersonaMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deletePersona(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["personas"] });
      queryClient.invalidateQueries({ queryKey: ["assignments"] });
    },
  });
}
